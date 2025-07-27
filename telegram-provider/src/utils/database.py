from functools import lru_cache

from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession

DATABASE_URL = "sqlite+aiosqlite:///./telegram.db"


@lru_cache()
def get_engine():
    return create_async_engine(DATABASE_URL, future=True, echo=False)


@lru_cache()
def get_session_maker():
    return async_sessionmaker(get_engine(), expire_on_commit=False, class_=AsyncSession)


async def get_session() -> AsyncSession:
    session_maker = get_session_maker()
    async with session_maker() as session:
        yield session
