import os
from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

ACCESS_KEY = "xxx"
SECRET_KEY = "xxx"
BASE_DIR = Path("local_storage")
BASE_DIR.mkdir(exist_ok=True)

GOOGLE_DRIVE_SA = os.getenv("GOOGLE_DRIVE_SA")

MIN_PASSWORD_LENGTH = 8  # todo увеличить в несколько раз для безопасности
ACCESS_KEY_LENGTH = 20
ACCESS_SECRET_LENGTH = 80

CHUNK_SIZE = 1024 * 1024 * 100  # 100 Mb


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
