import pytest
import pytest_asyncio

import os

from unittest.mock import Mock
from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine, async_sessionmaker

from src.utils.config import get_db_settings
from src.utils.database import Base
from src.utils.unitofwork import UnitOfWork


@pytest_asyncio.fixture(scope="function")
async def uow_test(db_session_test):
    return UnitOfWork(Mock(return_value=db_session_test))


@pytest_asyncio.fixture(scope="function")
async def db_session_test():
    engine = create_async_engine(
        get_db_settings().db_url, connect_args={"target_session_attrs": "read-write"}
    )
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    session_maker = async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)

    async with session_maker() as session:
        try:
            yield session
        finally:
            await session.rollback()
            async with engine.begin() as conn:
                await conn.run_sync(Base.metadata.drop_all)

    await engine.dispose()


@pytest.fixture(scope="session", autouse=True)
def set_required_envs():
    os.environ["BASE_URL"] = "http://localhost:3000"
    os.environ["ENV"] = "dev"

    os.environ["DB_NAME"] = "postgres"
    os.environ["DB_USER"] = "postgres"
    os.environ["DB_PASS"] = "postgres"
    os.environ["DB_HOST"] = os.getenv("DB_HOST", "localhost")
    os.environ["DB_PORT"] = os.getenv("DB_PORT", "5433")
