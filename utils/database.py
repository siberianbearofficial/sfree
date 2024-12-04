from functools import lru_cache

from sqlalchemy import MetaData
from sqlalchemy.orm import declarative_base
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession


from utils.config import DBSettings

Base = declarative_base()

metadata = MetaData()


@lru_cache()
def get_engine():
    return create_async_engine(
        DBSettings().db_url, connect_args={"target_session_attrs": "read-write"}
    )


@lru_cache()
def get_session_maker():
    return async_sessionmaker(get_engine(), class_=AsyncSession, expire_on_commit=False)


def get_session():
    session_maker = get_session_maker()
    with session_maker() as session:
        yield session
