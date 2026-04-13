import pytest

from httpx import AsyncClient, ASGITransport
from main import app


@pytest.fixture
def transport():
    return ASGITransport(app=app)


@pytest.mark.asyncio
async def test_get_root_handler(transport):
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/")
        assert response.status_code == 200
        assert response.json() == {
            "data": "SFree API",
            "detail": "Visit /docs or /redoc for the full documentation.",
        }


@pytest.mark.asyncio
async def test_get_readyz_handler(transport):
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/readyz")
        assert response.status_code == 200
        assert response.json() == {"data": "Ready", "detail": "API is ready."}


@pytest.mark.asyncio
async def test_get_healthz_handler(transport):
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/healthz")
        assert response.status_code == 200
        assert response.json() == {"data": "Healthy", "detail": "API is healthy."}


@pytest.mark.asyncio
async def test_get_publication_ready_handler(transport):
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/publication/ready")
        assert response.status_code == 200
        assert response.json() == {"data": "Ready", "detail": "API is ready to be published."}
