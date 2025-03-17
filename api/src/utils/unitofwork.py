from abc import ABC, abstractmethod
from functools import lru_cache

from sqlalchemy.ext.asyncio import AsyncSession

from src.utils.database import get_session_maker


class IUnitOfWork(ABC):

    @abstractmethod
    def __init__(self, session_factory):
        self.__session_factory = None
        self.session: AsyncSession

    @abstractmethod
    async def __aenter__(self):
        self.session = None

    @abstractmethod
    async def __aexit__(self, *args): ...

    @abstractmethod
    async def commit(self): ...

    @abstractmethod
    async def rollback(self): ...


class UnitOfWork(IUnitOfWork):
    def __init__(self, session_factory=None):
        self.__session_factory = session_factory or get_session_maker()

    async def __aenter__(self):
        self.session: AsyncSession = self.__session_factory()

    async def __aexit__(self, *args):
        await self.session.close()

    async def commit(self):
        await self.session.commit()

    async def rollback(self):
        await self.session.rollback()


@lru_cache
def get_uow() -> IUnitOfWork:
    return UnitOfWork()
