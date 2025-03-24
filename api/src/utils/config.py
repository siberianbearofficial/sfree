from functools import lru_cache

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


MIN_PASSWORD_LENGTH = 8  # todo увеличить в несколько раз для безопасности
ACCESS_SECRET_LENGTH = 80

CHUNK_SIZE = 1024 * 1024 * 100  # 100 Mb
ENV_FILE_PATH = "local.env"
LOCAL_INIT_VAR = "LOCAL_INIT"


class DBSettings(BaseSettings):
    name: str = ""
    user: str = ""
    password: str = Field(alias="db_pass")
    host: str = ""
    port: int = 0

    @property
    def access_str(self) -> str:
        return f"{self.user}:{self.password}@/{self.name}?host={self.host}:{self.port}"

    @property
    def db_url(self) -> str:
        return f"postgresql+asyncpg://{self.access_str}"

    model_config = SettingsConfigDict(env_prefix="db_")


@lru_cache
def get_db_settings() -> DBSettings:
    return DBSettings()  # type: ignore
