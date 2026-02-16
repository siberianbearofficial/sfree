import os
from dataclasses import dataclass
from uuid import uuid4

import pytest
import pytest_asyncio
from aiohttp import BasicAuth

from e2e.s3aas_client import E2EConfig, S3AASClient


@pytest.fixture(scope="session")
def e2e_config() -> E2EConfig:
    base_api_url = os.getenv("E2E_BASE_API_URL", "http://localhost:8080")
    source_type = os.getenv("E2E_SOURCE_TYPE", "gdrive").strip().lower()
    gdrive_key = os.getenv("E2E_GDRIVE_KEY", "")
    telegram_token = os.getenv("E2E_TELEGRAM_TOKEN", "")
    telegram_chat_id = os.getenv("E2E_TELEGRAM_CHAT_ID", "")

    if source_type == "gdrive" and not gdrive_key:
        raise RuntimeError("E2E_GDRIVE_KEY is required for gdrive e2e tests")
    if source_type == "telegram":
        if not telegram_token:
            raise RuntimeError("E2E_TELEGRAM_TOKEN is required for telegram e2e tests")
        if not telegram_chat_id:
            raise RuntimeError("E2E_TELEGRAM_CHAT_ID is required for telegram e2e tests")
    if source_type not in {"gdrive", "telegram"}:
        raise RuntimeError("E2E_SOURCE_TYPE must be either 'gdrive' or 'telegram'")

    return E2EConfig(
        base_api_url=base_api_url.rstrip("/"),
        source_type=source_type,
        gdrive_key=gdrive_key,
        telegram_token=telegram_token,
        telegram_chat_id=telegram_chat_id,
    )


@pytest_asyncio.fixture
async def client(e2e_config: E2EConfig) -> S3AASClient:
    client = S3AASClient(e2e_config)
    await client.wait_ready()
    try:
        yield client
    finally:
        await client.close()


@dataclass(slots=True)
class E2EContext:
    auth: BasicAuth
    bucket_id: str
    bucket_key: str
    access_key: str
    access_secret: str
    source_id: str
    source_type: str


@pytest_asyncio.fixture
async def e2e_context(client: S3AASClient) -> E2EContext:
    username = f"e2e-user-{uuid4().hex[:10]}"
    bucket_key = f"e2e-bucket-{uuid4().hex[:10]}"
    source_name = f"e2e-source-{uuid4().hex[:10]}"

    user = await client.create_user(username)
    auth = BasicAuth(login=username, password=user["password"])

    bucket = await client.create_bucket(auth=auth, key=bucket_key)
    buckets = await client.list_buckets(auth)
    bucket_id = next(item["id"] for item in buckets if item["key"] == bucket_key)

    source = await client.create_source(auth, source_name)

    try:
        yield E2EContext(
            auth=auth,
            bucket_id=bucket_id,
            bucket_key=bucket_key,
            access_key=bucket["access_key"],
            access_secret=bucket["access_secret"],
            source_id=source["id"],
            source_type=client.config.source_type,
        )
    finally:
        files = await client.list_files(auth, bucket_id)
        for file_doc in files:
            await client.delete_file(auth, bucket_id, file_doc["id"])
        await client.delete_source(auth, source["id"])
        await client.delete_bucket(auth, bucket_id)
