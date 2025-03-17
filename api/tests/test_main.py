import pytest

from httpx import AsyncClient
from main import app


@pytest.mark.asyncio
async def test_get_root_handler():
    async with AsyncClient(app=app, base_url="http://test") as ac:
        response = await ac.get("/")
    assert response.status_code == 200
    assert response.json() == {
        "data": "S3aaS API",
        "detail": "Visit /docs or /redoc for the full documentation.",
    }


@pytest.mark.asyncio
async def test_get_readyz_handler():
    async with AsyncClient(app=app, base_url="http://test") as ac:
        response = await ac.get("/readyz")
    assert response.status_code == 200
    assert response.json() == {"data": "Ready", "detail": "API is ready."}


@pytest.mark.asyncio
async def test_get_healthz_handler():
    async with AsyncClient(app=app, base_url="http://test") as ac:
        response = await ac.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"data": "Healthy", "detail": "API is healthy."}


@pytest.mark.asyncio
async def test_get_publication_ready_handler():
    async with AsyncClient(app=app, base_url="http://test") as ac:
        response = await ac.get("/publication/ready")
    assert response.status_code == 200
    assert response.json() == {"data": "Ready", "detail": "API is ready to be published."}
