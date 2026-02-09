import os

import pytest
import pytest_asyncio

from e2e.s3aas_client import E2EConfig, S3AASClient


@pytest.fixture(scope="session")
def e2e_config() -> E2EConfig:
    base_api_url = os.getenv("E2E_BASE_API_URL", "http://localhost:8080")
    gdrive_key = os.getenv("E2E_GDRIVE_KEY", "")
    if not gdrive_key:
        raise RuntimeError("E2E_GDRIVE_KEY is required for e2e tests")
    return E2EConfig(base_api_url=base_api_url.rstrip("/"), gdrive_key=gdrive_key)


@pytest_asyncio.fixture
async def client(e2e_config: E2EConfig) -> S3AASClient:
    client = S3AASClient(e2e_config)
    await client.wait_ready()
    try:
        yield client
    finally:
        await client.close()
