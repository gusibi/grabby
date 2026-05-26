"""
MCP 工具 - WebSocketManager
- 连接管理机制
- 基于消息 ID 的请求-响应模式
- 异步通信实现
- 错误处理和资源清理
"""

from typing import Dict, Optional, List
import uuid
import asyncio

from fastapi import WebSocket
from config import settings

from logger import get_logger

logger = get_logger(__name__)

class WebSocketManager:
    def __init__(self):
        self.active_connections: Dict[str, WebSocket] = {}  # {conn_id: websocket}
        self.pending_responses: Dict[str, asyncio.Future] = {}  # 存储待响应的 Future
        self.browser_names: Dict[str, str] = {}  # {conn_id: name}

    async def connect(self, websocket: WebSocket, conn_id: Optional[str] = None) -> str:
        logger.debug("正在接受 WebSocket 连接...")
        await websocket.accept()
        conn_id = conn_id or str(uuid.uuid4())  # 如果没有提供 conn_id，则生成一个
        self.active_connections[conn_id] = websocket
        logger.info(f"新连接建立，conn_id: {conn_id}")
        logger.debug(f"当前连接数: {len(self.active_connections)}")
        return conn_id

    def disconnect(self, conn_id: str):
        logger.debug(f"正在断开 WebSocket 连接..., 当前连接数: {len(self.active_connections)}")
        if conn_id in self.active_connections:
            self.active_connections.pop(conn_id)
            logger.info(f"连接断开，conn_id: {conn_id}")
        logger.debug(f"已断开 WebSocket 连接，当前连接数: {len(self.active_connections)}")

    def has_connection(self, conn_id: str) -> bool:
        """检查连接是否已存在"""
        return conn_id in self.active_connections

    async def send_message(
        self,
        message: dict,
        target_conn_id: Optional[str] = None,
        timeout: Optional[float] = None
    ) -> dict:
        """
        发送消息到指定连接（默认发送到第一个可用连接）
        - target_conn_id: 可指定目标连接的 conn_id
        """
        if not self.active_connections:
            raise ConnectionError("没有活动的 WebSocket 连接")
        logger.debug(f"正在发送消息, target_conn_id: {target_conn_id}, message: {message}")

        # 如果没有指定 conn_id，默认选择第一个连接
        websocket = (
            self.active_connections.get(target_conn_id)
            if target_conn_id
            else next(iter(self.active_connections.values()))
        )

        if not websocket:
            raise ConnectionError(f"未找到目标连接: {target_conn_id}")

        if not message.get("message_id", ""):
            # 如果消息中未包含 message_id, 则生产一个
            message_id = str(uuid.uuid4())
            message["message_id"] = message_id  # 加入唯一消息 ID
        else:
            message_id = message["message_id"]

        logger.debug(f"new message: {message}")
        future = asyncio.get_event_loop().create_future()
        self.pending_responses[message_id] = future

        try:
            await websocket.send_json(message)
            response = await asyncio.wait_for(future, timeout=timeout or settings.websocket_timeout)
            return response
        except asyncio.TimeoutError:
            raise ConnectionError("等待响应超时")
        finally:
            self.pending_responses.pop(message_id, None)

    async def handle_response(self, data: dict):
        """处理 Postman 返回的响应"""
        message_id = data.get("message_id")
        logger.debug(f"开始响应: {data}, pending_responses: {self.pending_responses}")
        if message_id in self.pending_responses:
            future = self.pending_responses[message_id]
            if not future.done():
                future.set_result(data)  # 通知 `send_message` 已收到响应

    def register_browser_name(self, conn_id: str, name: str):
        """注册浏览器名称映射"""
        if not name:
            raise ValueError("浏览器名称不能为空")
        if self.is_browser_name_active(name, exclude_conn_id=conn_id):
            raise ValueError(f"浏览器名称已连接: {name}")
        self.browser_names[conn_id] = name
        logger.info(f"浏览器已注册: conn_id={conn_id}, name={name}")

    def unregister_browser_name(self, conn_id: str):
        """注销浏览器名称映射"""
        self.browser_names.pop(conn_id, None)

    def get_browser_list(self) -> List[Dict[str, str]]:
        """获取已连接的浏览器列表"""
        return [
            {"conn_id": conn_id, "name": self.browser_names.get(conn_id, "")}
            for conn_id in self.browser_names.keys()
            if conn_id in self.active_connections
        ]

    def is_browser_name_active(self, name: str, exclude_conn_id: Optional[str] = None) -> bool:
        """检查浏览器名称是否已有活动连接"""
        for conn_id, active_name in self.browser_names.items():
            if conn_id != exclude_conn_id and active_name == name and conn_id in self.active_connections:
                return True
        return False

    def resolve_browser_conn_id(self, name: Optional[str] = None) -> str:
        """根据浏览器名称解析连接ID

        name 为空时使用默认浏览器（配置的 default_browser 或第一个连接）
        """
        if not self.active_connections:
            raise ConnectionError("没有活动的浏览器连接")

        if not name:
            # 尝试使用配置的默认浏览器
            if settings.default_browser:
                for conn_id, n in self.browser_names.items():
                    if n == settings.default_browser and conn_id in self.active_connections:
                        return conn_id
            # 返回第一个活动连接
            for conn_id in self.browser_names.keys():
                if conn_id in self.active_connections:
                    return conn_id
            raise ConnectionError("没有活动的浏览器连接")

        # 按名称查找
        for conn_id, n in self.browser_names.items():
            if n == name and conn_id in self.active_connections:
                return conn_id

        raise ConnectionError(f"未找到浏览器 '{name}'")
