from typing import Optional
import uvicorn
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, HTTPException, Query, status
from fastapi_mcp import add_mcp_server
from pydantic import BaseModel
from datetime import datetime
from websocket_manager import WebSocketManager
from browser_registry import BrowserRegistry, BrowserRegistryError
from config import settings
from logger import get_logger

logger = get_logger(__name__)

"""
Grabby 工具服务器 - 主模块

本模块实现了MCP工具的服务器端功能，包括：
- FastAPI 应用结构：提供Web API和WebSocket端点
- WebSocket 通信实现：处理浏览器和命令客户端的连接
- MCP 服务器集成：集成fastapi_mcp库，提供MCP协议支持
- 工具函数实现：提供截图等功能的API端点
"""

# 创建FastAPI应用实例
app = FastAPI(
    title="Grabby 工具服务器",
    description="通过真实浏览器抓取网页内容并转换为 Markdown 的服务",
    version="1.0.0"
)

# 创建WebSocket连接管理器实例
ws_manager = WebSocketManager()
browser_registry = BrowserRegistry()

@app.websocket("/ws_browser")
async def websocket_browser(websocket: WebSocket, conn_id: Optional[str] = None):
    """
    浏览器WebSocket连接端点

    处理来自浏览器扩展的WebSocket连接，接收浏览器发送的消息并处理响应

    参数:
        websocket: WebSocket连接对象
        conn_id: 可选的连接ID，如果未提供则自动生成
    """
    browser_name = websocket.query_params.get("name", "").strip()
    conn_id = (conn_id or "").strip()

    if not conn_id or not browser_name:
        logger.warning("浏览器WebSocket连接被拒绝：缺少 conn_id 或 name")
        await websocket.close(code=4001, reason="Missing conn_id or name")
        return

    if not browser_registry.validate(conn_id, browser_name):
        logger.warning(f"浏览器WebSocket连接被拒绝：未注册或名称不匹配 conn_id={conn_id}, name={browser_name}")
        await websocket.close(code=4001, reason="Browser is not registered")
        return

    if ws_manager.has_connection(conn_id):
        logger.warning(f"浏览器WebSocket连接被拒绝：conn_id 已连接 [{conn_id}]")
        await websocket.close(code=4003, reason="Browser id already connected")
        return

    if ws_manager.is_browser_name_active(browser_name):
        logger.warning(f"浏览器WebSocket连接被拒绝：name 已连接 [{browser_name}]")
        await websocket.close(code=4003, reason="Browser name already connected")
        return

    conn_id = await ws_manager.connect(websocket, conn_id)
    try:
        ws_manager.register_browser_name(conn_id, browser_name)
    except ValueError as e:
        logger.warning(f"浏览器WebSocket连接被拒绝：{e}")
        await websocket.close(code=4003, reason=str(e))
        ws_manager.disconnect(conn_id)
        return
    logger.info(f"浏览器WebSocket连接已建立 [{conn_id}] name={browser_name}")

    try:
        while True:
            # 接收浏览器发送的消息
            data = await websocket.receive_json()

            # 根据消息类型进行处理
            if "message_id" in data:
                # 如果是响应消息，则传递给响应处理器
                logger.info(f"浏览器 [{conn_id}] 发送响应: message_id={data['message_id']}")
                await ws_manager.handle_response(data)
            else:
                # 处理其他类型的消息（如心跳包等）
                logger.debug(f"浏览器 [{conn_id}] 发送其他消息: {data}")
    except WebSocketDisconnect:
        # 处理WebSocket断开连接
        logger.info(f"浏览器 [{conn_id}] 断开连接")
    except Exception as e:
        # 处理其他异常
        logger.error(f"浏览器WebSocket连接 [{conn_id}] 发生异常: {str(e)}", exc_info=True)
    finally:
        ws_manager.unregister_browser_name(conn_id)
        ws_manager.disconnect(conn_id)

@app.websocket("/ws_command")
async def websocket_send_command(websocket: WebSocket, conn_id: Optional[str] = None):
    """
    命令WebSocket连接端点
    
    处理来自命令客户端的WebSocket连接，接收命令并转发给浏览器执行
    
    参数:
        websocket: WebSocket连接对象
        conn_id: 可选的连接ID，如果未提供则自动生成
    """
    # 建立WebSocket连接
    if conn_id:
        conn_id = f"ws_command:{conn_id}"
    conn_id = await ws_manager.connect(websocket, conn_id)
    logger.info(f"命令WebSocket连接已建立 [{conn_id}]")
    
    try:
        while True:
            # 接收客户端发送的命令
            data = await websocket.receive_json()
            logger.info(f"命令客户端 [{conn_id}] 发送命令: {data.get('command', '未知命令')}, URL: {data.get('url', '')}")
            
            # 验证命令格式
            if "command" in data and "url" in data:
                # 构造消息并发送给浏览器
                message = {
                    "source": data.get("source","ws_command"),
                    "action": data.get("action", data.get("command")),  # 兼容不同格式
                    "command": data["command"],
                    "url": data["url"],
                    "fullPage": data.get("fullPage", False),
                    "message_id": data.get("message_id", "")
                }

                try:
                    # 根据 browser 名称解析目标连接
                    browser_name = data.get("browser", "")
                    browser_conn_id = ws_manager.resolve_browser_conn_id(browser_name)
                    logger.info(f"转发命令到浏览器 [{browser_conn_id}] name={browser_name}: message_id={message['message_id']}")
                    response = await ws_manager.send_message(message, target_conn_id=browser_conn_id)
                    
                    # 将响应发送回客户端
                    logger.info(f"收到浏览器响应并转发回命令客户端 [{conn_id}]")
                    await websocket.send_json(response)
                except ConnectionError as e:
                    # 处理连接错误
                    error_msg = {"error": str(e), "status": "error"}
                    logger.error(f"命令执行失败: {str(e)}")
                    await websocket.send_json(error_msg)
            else:
                # 处理格式错误的命令
                error_msg = {"error": "无效的命令格式，需要包含 'command' 和 'url' 字段", "status": "error"}
                logger.warning(f"收到格式错误的命令: {data}")
                await websocket.send_json(error_msg)
    except WebSocketDisconnect:
        # 处理WebSocket断开连接
        logger.info(f"命令客户端 [{conn_id}] 断开连接")
        ws_manager.disconnect(conn_id)
    except Exception as e:
        # 处理其他异常
        logger.error(f"命令WebSocket连接 [{conn_id}] 发生异常: {str(e)}")
        ws_manager.disconnect(conn_id)

# Pydantic 请求模型
class ExtractRequest(BaseModel):
    url: str
    browser: Optional[str] = None  # 浏览器名称，为空时使用默认浏览器


class ScreenshotRequest(BaseModel):
    url: str
    fullPage: bool = False
    browser: Optional[str] = None


class BrowserRegisterRequest(BaseModel):
    connect_id: str
    name: str


# 配置并添加MCP服务器
mcp_server = add_mcp_server(
    app,                                # FastAPI应用实例
    mount_path="/mcp",                  # MCP服务器挂载路径
    name="Grabby",                # MCP服务器名称
    describe_all_responses=True,        # 在工具描述中包含所有可能的响应模式
    describe_full_response_schema=True  # 在工具描述中包含完整的JSON模式
)

@mcp_server.tool()
async def screenshot(url: str, fullPage=False, browser="") -> str:
    """
    捕获指定URL的网页截图

    通过浏览器扩展捕获指定URL的网页截图，并返回Base64编码的图片数据

    参数:
        url: 要截图的网页URL
        fullPage: 是否捕获整个页面，默认为False
        browser: 浏览器名称（可选，为空时使用默认浏览器）

    返回:
        Base64编码的图片数据字符串
    """
    try:
        conn_id = ws_manager.resolve_browser_conn_id(browser or None)
    except ConnectionError as e:
        logger.error(f"浏览器不可用: {e}")
        return f"浏览器不可用: {str(e)}"

    try:
        # 构造截图命令并发送到浏览器
        logger.info(f"执行网页截图: {url} browser={browser}")
        response = await ws_manager.send_message({
            "source": "mcp_client",
            "action": "mcp_request",
            "command": "capture",
            "url": url,
            "fullPage": fullPage,
        }, target_conn_id=conn_id)
        
        # 检查响应中是否包含图片数据
        if response.get("result", {}).get("imageData", ""):
            logger.info(f"成功获取网页截图: {url}")
            return response.get("result", {}).get("imageData", "")
        else:
            logger.warning(f"截图响应中未包含图片数据: {response}")
            return ""
    except ConnectionError as e:
        logger.error(f"截图操作失败: {str(e)}")
        return f"截图失败: {str(e)}"

@mcp_server.tool()
async def extract(url: str, browser="") -> str:
    """
    提取指定URL的网页内容

    通过浏览器扩展提取指定URL的网页内容，并返回 Markdown 格式的文本数据

    参数:
        url: 要提取内容的网页URL
        browser: 浏览器名称（可选，为空时使用默认浏览器）
    返回:
        提取的 Markdown 文本数据字符串
    """
    try:
        conn_id = ws_manager.resolve_browser_conn_id(browser or None)
    except ConnectionError as e:
        logger.error(f"浏览器不可用: {e}")
        return f"浏览器不可用: {str(e)}"

    try:
        # 发送提取命令到浏览器
        logger.info(f"执行网页内容抓取: {url} browser={browser}")
        response = await ws_manager.send_message({
            "source": "mcp_client",
            "action": "mcp_request",
            "command": "extract",
            "url": url,
        }, target_conn_id=conn_id)

        result = response.get("result", {})
        content = result.get("content", {})
        # 浏览器扩展已使用 defuddle 将内容转换为 Markdown，直接返回
        markdown = content.get("content", "") if isinstance(content, dict) else ""
        if markdown:
            logger.info(f"成功获取 Markdown 内容: {url}")
            return markdown
        else:
            logger.warning(f"extract 响应中未包含内容数据: {response}")
            return ""
    except ConnectionError as e:
        logger.error(f"extract 操作失败: {str(e)}")
        return f"extract 失败: {str(e)}"


@mcp_server.tool()
async def add(a: int, b: int) -> int:
    """
    计算两个数字的和
    
    参数:
        a: 第一个数字
        b: 第二个数字
        
    返回:
        两个数字的和
    """
    result = a + b
    logger.debug(f"计算: {a} + {b} = {result}")
    return result

@mcp_server.tool()
async def get_server_time() -> str:
    """
    获取服务器当前时间
    
    返回:
        ISO格式的服务器当前时间字符串
    """
    current_time = datetime.now().isoformat()
    logger.debug(f"获取服务器时间: {current_time}")
    return current_time



@app.get("/api/health")
async def health_check():
    """健康检查端点"""
    browsers = ws_manager.get_browser_list()
    return {
        "status": "ok",
        "browser_connected": len(browsers) > 0,
        "browser_count": len(browsers),
        "browsers": browsers,
        "timestamp": datetime.now().isoformat(),
    }


@app.get("/api/browsers")
async def list_browsers():
    """获取已连接的浏览器列表"""
    browsers = ws_manager.get_browser_list()
    return {
        "browsers": browsers,
        "count": len(browsers),
    }


@app.post("/api/browsers/register")
async def register_browser(request: BrowserRegisterRequest):
    """注册浏览器实例，浏览器 WebSocket 连接前必须先注册"""
    try:
        browser = browser_registry.register(request.connect_id, request.name)
    except BrowserRegistryError as e:
        logger.warning(f"浏览器注册失败: {e}")
        raise HTTPException(status_code=409, detail=str(e))

    return {
        "success": True,
        "browser": browser,
    }


@app.post("/api/extract")
async def api_extract(request: ExtractRequest):
    """
    提取指定 URL 的网页内容并返回 Markdown

    浏览器扩展已使用 defuddle 将内容转换为 Markdown，服务端直接返回

    - **url**: 要提取的网页 URL
    - **browser**: 浏览器名称（可选，为空时使用默认浏览器）
    """
    try:
        browser_conn_id = ws_manager.resolve_browser_conn_id(request.browser)
    except ConnectionError as e:
        logger.warning(f"浏览器不可用: {e}")
        raise HTTPException(status_code=503, detail=str(e))

    try:
        logger.info(f"HTTP API 请求提取: {request.url} browser={request.browser}")
        response = await ws_manager.send_message(
            {
                "source": "http_api",
                "action": "mcp_request",
                "command": "extract",
                "url": request.url,
            },
            target_conn_id=browser_conn_id,
            timeout=settings.api_extract_timeout,
        )

        if not response.get("success"):
            error_msg = response.get("error", "提取失败")
            logger.error(f"浏览器扩展返回错误: {error_msg}")
            raise HTTPException(status_code=502, detail=f"浏览器扩展错误: {error_msg}")

        result = response.get("result", {})
        content = result.get("content", {})

        # 浏览器扩展已使用 defuddle 将内容转换为 Markdown，直接取 content 字段
        markdown = content.get("content", "") if isinstance(content, dict) else ""

        return {
            "success": True,
            "url": result.get("url", request.url),
            "title": result.get("title", "") or content.get("title", ""),
            "markdown": markdown,
        }

    except ConnectionError as e:
        logger.error(f"提取操作连接错误: {e}")
        raise HTTPException(status_code=504, detail=f"提取超时或连接断开: {str(e)}")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"提取操作失败: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"服务器内部错误: {str(e)}")


@app.post("/api/screenshot")
async def api_screenshot(request: ScreenshotRequest):
    """
    网页截图并返回 Base64 编码的图片数据

    - **url**: 要截图的网页 URL
    - **fullPage**: 是否截取整页 (默认 False)
    - **browser**: 浏览器名称（可选，为空时使用默认浏览器）
    """
    try:
        browser_conn_id = ws_manager.resolve_browser_conn_id(request.browser)
    except ConnectionError as e:
        logger.warning(f"浏览器不可用: {e}")
        raise HTTPException(status_code=503, detail=str(e))

    try:
        logger.info(f"HTTP API 请求截图: {request.url} browser={request.browser} fullPage={request.fullPage}")
        response = await ws_manager.send_message(
            {
                "source": "http_api",
                "action": "mcp_request",
                "command": "capture",
                "url": request.url,
                "fullPage": request.fullPage,
            },
            target_conn_id=browser_conn_id,
            timeout=settings.api_extract_timeout,
        )

        if not response.get("success"):
            error_msg = response.get("error", "截图失败")
            logger.error(f"浏览器扩展返回错误: {error_msg}")
            raise HTTPException(status_code=502, detail=f"浏览器扩展错误: {error_msg}")

        result = response.get("result", {})
        image_data = result.get("imageData", "")

        return {
            "success": True,
            "url": result.get("url", request.url),
            "imageData": image_data,
        }

    except ConnectionError as e:
        logger.error(f"截图操作连接错误: {e}")
        raise HTTPException(status_code=504, detail=f"截图超时或连接断开: {str(e)}")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"截图操作失败: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"服务器内部错误: {str(e)}")


if __name__ == "__main__":
    # 打印所有注册的路由信息（仅在调试模式下）
    if settings.debug:
        for route in app.routes:
            logger.debug(f"注册路由: {route.path} - {route.name}")

    # 启动服务器
    logger.info(f"启动 Grabby 工具服务器 - 监听: {settings.host}:{settings.port}")
    uvicorn.run(
        app,
        host=settings.host,
        port=settings.port,
        log_level="info"
    )
