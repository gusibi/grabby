from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    """服务器配置

    所有配置项优先从 .env 文件读取，缺失时使用以下默认值。
    环境变量名与字段名相同（不区分大小写）。
    """
    host: str = "0.0.0.0"
    port: int = 5040
    connect_id: str = "browser-tools"
    debug: bool = False
    websocket_timeout: float = 5.0
    api_extract_timeout: float = 60.0
    default_browser: str = ""  # 默认浏览器名称，空字符串表示使用第一个连接

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False
        extra = "ignore"

settings = Settings()