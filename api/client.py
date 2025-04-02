import asyncio
import sys

from loguru import logger
from aiohttp import ClientSession, BasicAuth
from aiobotocore.session import get_session
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    base_api_url: str = "http://localhost:8000"
    gdrive_key: str  # Секретный ключ для Google Drive
    username: str = "testuser"
    bucket_key: str = "test-bucket"

    @property
    def user_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/users"

    @property
    def bucket_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/buckets"

    @property
    def s3_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/s3"

    @property
    def gdrive_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/sources/gdrive"


async def create_user(session: ClientSession, settings: Settings) -> dict:
    async with session.post(settings.user_endpoint, json={"username": settings.username}) as resp:
        data = await resp.json()
        logger.info("User created: {}", data)
        return data["data"]


async def create_bucket(session: ClientSession, auth: BasicAuth, settings: Settings) -> dict:
    async with session.post(
        settings.bucket_endpoint, json={"key": settings.bucket_key}, auth=auth
    ) as resp:
        data = await resp.json()
        logger.info("Bucket created: {}", data)
        return data["data"]


async def add_gdrive_source(
    session: ClientSession, auth: BasicAuth, settings: Settings, name: str = "Test GDrive Source"
) -> dict:
    payload = {"key": settings.gdrive_key, "name": name}
    async with session.post(settings.gdrive_endpoint, json=payload, auth=auth) as resp:
        data = await resp.json()
        logger.info("GDrive source added")
        return data["data"]


async def upload_file_s3(client, bucket_key: str, file_key: str, content: bytes) -> None:
    await client.put_object(Bucket=bucket_key, Key=file_key, Body=content)
    logger.info("Uploaded file to S3")


async def download_file_s3(client, bucket_key: str, file_key: str) -> bytes:
    response = await client.get_object(Bucket=bucket_key, Key=file_key)
    content = await response["Body"].read()
    logger.info("Downloaded file from S3")
    return content


async def run_e2e_test(settings: Settings) -> None:
    test_file_content = b"This is a test file content"
    test_file_key = "test_file.txt"

    async with ClientSession(raise_for_status=True) as http_session:
        # Шаг 1: Создание пользователя
        user_data = await create_user(http_session, settings)
        user_password = user_data.get("password")
        if not user_password:
            raise Exception("User password missing")
        auth = BasicAuth(login=settings.username, password=user_password)

        # Шаг 2: Создание бакета
        bucket_data = await create_bucket(http_session, auth, settings)
        if "access_key" not in bucket_data or "access_secret" not in bucket_data:
            raise Exception("Bucket credentials missing")

        # Шаг 3: Добавление Google Drive Source
        await add_gdrive_source(http_session, auth, settings)

        # Шаг 4: Загрузка и скачивание файла через S3
        s3_session = get_session()
        async with s3_session.create_client(
            "s3",
            region_name="us-east-1",
            endpoint_url=settings.s3_endpoint,
            aws_secret_access_key=bucket_data["access_secret"],
            aws_access_key_id=bucket_data["access_key"],
            verify=False,
        ) as s3_client:
            await upload_file_s3(
                s3_client,
                bucket_data.get("key", settings.bucket_key),
                test_file_key,
                test_file_content,
            )
            downloaded_content = await download_file_s3(
                s3_client, bucket_data.get("key", settings.bucket_key), test_file_key
            )
            if downloaded_content != test_file_content:
                raise Exception("File content mismatch")

    logger.info("E2E test passed successfully.")


async def main() -> int:
    # Загружаем конфигурацию из переменных окружения
    settings = Settings()  # type: ignore
    try:
        await run_e2e_test(settings)
        return 0
    except Exception as e:
        logger.exception("E2E test failed: {}", e)
        return 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
