from functools import lru_cache
import os

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


MIN_PASSWORD_LENGTH = 8  # todo увеличить в несколько раз для безопасности
ACCESS_SECRET_LENGTH = 80

CHUNK_SIZE = 1024 * 1024 * 100  # 100 Mb
ENV_FILE_PATH = "local.env"
LOCAL_INIT_VAR = "LOCAL_INIT"


class DBSettings(BaseSettings):
    name: str = "postgres"
    user: str = "postgres"
    password: str = Field(default="postgres", alias="db_pass")
    host: str = "localhost"
    port: int = 5432

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


if os.getenv(LOCAL_INIT_VAR) == "1":
    import dotenv

    if dotenv.load_dotenv(dotenv_path=ENV_FILE_PATH, override=True):
        DBSettings.name = os.getenv("DB_NAME", DBSettings.name)
        DBSettings.user = os.getenv("DB_USER", DBSettings.user)
        DBSettings.password = os.getenv("DB_PASS", DBSettings.password)
        DBSettings.host = os.getenv("DB_HOST", DBSettings.host)
        DBSettings.port = int(os.getenv("DB_PORT", DBSettings.port))
