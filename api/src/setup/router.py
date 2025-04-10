import subprocess
from typing import Annotated

from fastapi import APIRouter, Path
from loguru import logger

from src.setup.response import (
    PostMigrationsUpgradeResponse,
    GetPublicationReadyResponse,
    GetHealthResponse,
    GetReadyResponse,
    GetRootResponse,
)

# from utils.admin_auth import AdminAuthDep  # todo добавить админскую авторизацию

router = APIRouter()


@router.get(
    "/",
    summary="Basic API info",
    description="Основная информация о приложении",
    response_model=GetRootResponse,
)
async def get_root_handler():
    return GetRootResponse()


@router.get(
    "/readyz",
    summary="Readiness check",
    description="Возвращает 200, когда приложение готово принимать запросы",
    response_model=GetReadyResponse,
)
async def get_readyz_handler():
    return GetReadyResponse(data="Ready")


@router.get(
    "/healthz",
    summary="Health check",
    description="Возвращает 200, когда приложение работает",
    response_model=GetHealthResponse,
)
async def get_healthz_handler():
    return GetHealthResponse(data="Healthy")


@router.get(
    "/publication/ready",
    summary="Publication readiness check",
    description="Возвращает 200, когда приложение готово к публикации",
    response_model=GetPublicationReadyResponse,
)
async def get_publication_ready_handler():
    return GetPublicationReadyResponse(data="Ready")


@router.post(
    "/api/v1/migrations/{name}/upgrade",
    summary="Database migration upgrade",
    description="Накатывает миграции вплоть до указанной ревизии на PostgreSQL с помощью alembic. Ожидается, что в основном ручка будет вызываться с `name=head`",
    response_model=PostMigrationsUpgradeResponse,
)
async def post_migrations_upgrade(
    name: Annotated[str, Path(description="Название ревизии", example="head")],
    # _: AdminAuthDep,
):
    try:
        alembic_output = subprocess.check_output(
            ["alembic", "-c", "migrations/alembic.ini", "upgrade", name],
            text=True,
            stderr=subprocess.STDOUT,
        )

        print(alembic_output)
        logger.info("Migration upgrade successful.")

        return PostMigrationsUpgradeResponse(data=alembic_output)
    except subprocess.CalledProcessError as e:
        logger.error("Failed to upgrade migrations.", exc_info=e)
        raise e
