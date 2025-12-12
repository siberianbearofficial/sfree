import asyncio
import sys
from typing import Any, Dict, List, Optional

from aiobotocore.session import get_session
from aiohttp import BasicAuth, ClientSession, FormData
from loguru import logger
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    base_api_url: str = "https://s3aas-api.dev.nachert.art"
    gdrive_key: str  # Секретный ключ для Google Drive
    username: str = "testuser4"
    bucket_key: str = "test-bucket2"

    @property
    def user_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/users"

    @property
    def bucket_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/buckets"

    @property
    def s3_endpoint(self) -> str:
        return f"{self.base_api_url}/api/s3"

    @property
    def gdrive_endpoint(self) -> str:
        return f"{self.base_api_url}/api/v1/sources/gdrive"

    def bucket_upload_endpoint(self, bucket_id: str) -> str:
        return f"{self.base_api_url}/api/v1/buckets/{bucket_id}/upload"


async def create_user(session: ClientSession, settings: Settings) -> dict:
    async with session.post(settings.user_endpoint, json={"username": settings.username}, ssl=False) as resp:
        data = await resp.json()
        logger.info("User created: {}", data)
        return data


async def create_bucket(session: ClientSession, auth: BasicAuth, settings: Settings) -> dict:
    # 1) создаём бакет
    async with session.post(
        settings.bucket_endpoint,
        json={"key": settings.bucket_key},
        auth=auth,
        ssl=False,
    ) as resp:
        created = await resp.json()

    # 2) получаем список бакетов и находим id по key
    async with session.get(settings.bucket_endpoint, auth=auth, ssl=False) as resp:
        buckets: List[Dict[str, Any]] = await resp.json()

    matched: Optional[Dict[str, Any]] = next(
        (b for b in buckets if b.get("key") == settings.bucket_key),
        None,
    )
    if not matched or not matched.get("id"):
        raise Exception(f"Created bucket not found in list by key={settings.bucket_key}")

    # 3) возвращаем данные создания + id
    result = dict[Any, Any](created)
    result["id"] = matched["id"]
    result.setdefault("key", matched.get("key"))
    result.setdefault("created_at", matched.get("created_at"))

    logger.info("Bucket created: {}", result)
    return result


async def add_gdrive_source(
    session: ClientSession, auth: BasicAuth, settings: Settings, name: str = "Test GDrive Source"
) -> dict:
    payload = {"key": settings.gdrive_key, "name": name}
    async with session.post(settings.gdrive_endpoint, json=payload, auth=auth, ssl=False) as resp:
        data = await resp.json()
        logger.info("GDrive source added")
        return data


async def upload_file_s3(client, bucket_key: str, file_key: str, content: bytes) -> None:
    await client.put_object(Bucket=bucket_key, Key=file_key, Body=content)
    logger.info("Uploaded file to S3")


async def download_file_s3(client, bucket_key: str, file_key: str) -> bytes:
    response = await client.get_object(Bucket=bucket_key, Key=file_key)
    content = await response["Body"].read()
    logger.info("Downloaded file from S3")
    return content


async def upload_file_http(
    session: ClientSession,
    auth: BasicAuth,
    settings: Settings,
    bucket_id: str,
    file_name: str,
    content: bytes,
    field_name: str = "file",
) -> dict:
    form = FormData()
    form.add_field(
        name=field_name,
        value=content,
        filename=file_name,
        content_type="application/octet-stream",
    )

    async with session.post(
        settings.bucket_upload_endpoint(bucket_id),
        data=form,
        auth=auth,
        ssl=False,
    ) as resp:
        data = await resp.json()
        logger.info("Uploaded file via HTTP: {}", data)
        return data


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

        bucket_id = bucket_data.get("id")
        if not bucket_id:
            raise Exception("Bucket id missing")

        # Шаг 3: Добавление Google Drive Source
        await add_gdrive_source(http_session, auth, settings)

        # Шаг 4: Загрузка файла через HTTP (НОВАЯ РУЧКА)
        await upload_file_http(
            session=http_session,
            auth=auth,
            settings=settings,
            bucket_id=bucket_id,
            file_name=test_file_key,
            content=test_file_content,
        )

        # Шаг 5: Скачивание файла через S3-like как и было
        s3_session = get_session()
        async with s3_session.create_client(
            "s3",
            region_name="us-east-1",
            endpoint_url=settings.s3_endpoint,
            aws_secret_access_key=bucket_data["access_secret"],
            aws_access_key_id=bucket_data["access_key"],
            verify=False,
        ) as s3_client:
            downloaded_content = await download_file_s3(
                s3_client,
                bucket_data.get("key", settings.bucket_key),
                test_file_key,
            )
            with open(f"downloaded-{test_file_key}", "wb") as f:
                f.write(downloaded_content)

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
