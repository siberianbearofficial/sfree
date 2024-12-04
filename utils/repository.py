from abc import ABC, abstractmethod
from typing import Optional
from uuid import UUID

from pydantic import BaseModel
from sqlalchemy import select, update, delete
from sqlalchemy.ext.asyncio import AsyncSession

from utils.model import Model


class IRepository(ABC):
    @abstractmethod
    async def add(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def edit(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def edit_all(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def get(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def get_all(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def delete(self, *args, **kwargs):
        raise NotImplementedError

    @abstractmethod
    async def delete_all(self, *args, **kwargs):
        raise NotImplementedError


class SQLAlchemyRepository(IRepository):
    model = Model

    async def add(self, session: AsyncSession, data: Model) -> UUID:
        session.add(data)
        await session.flush()
        return data.id

    async def edit(self, session: AsyncSession, id: UUID, data: dict) -> UUID:
        stmt = (
            update(self.model).values(**data).filter_by(id=id).returning(self.model.id)
        )
        res = await session.execute(stmt)
        return res.scalar_one()

    async def edit_all(
        self, session: AsyncSession, data: dict, **filter_by
    ) -> list[UUID]:
        stmt = (
            update(self.model)
            .values(**data)
            .filter_by(**filter_by)
            .returning(self.model.id)
        )
        res = await session.execute(stmt)
        return [row[0] for row in res.all()]

    async def get(self, session: AsyncSession, **filter_by) -> Optional[BaseModel]:
        stmt = select(self.model).filter_by(**filter_by).limit(1)
        res = await session.execute(stmt)
        res = [row[0].to_read_model() for row in res.all()]
        if res:
            return res[0]
        return None

    async def get_all(self, session: AsyncSession, **filter_by) -> list[BaseModel]:
        stmt = select(self.model).filter_by(**filter_by)
        res = await session.execute(stmt)
        return [row[0].to_read_model() for row in res.all()]

    async def delete(self, session: AsyncSession, id: UUID) -> UUID:
        stmt = delete(self.model).where(self.model.id == id).returning(self.model.id)
        return await session.execute(stmt)

    async def delete_all(self, session: AsyncSession, **filter_by):
        stmt = delete(self.model).filter_by(**filter_by)
        return await session.execute(stmt)


class TimestampRepository(SQLAlchemyRepository):
    async def get_all(self, session: AsyncSession, **filter_by) -> list[BaseModel]:
        stmt = (
            select(self.model)
            .filter_by(**filter_by)
            .order_by(self.model.created_at.desc())
        )

        res = await session.execute(stmt)
        return [row[0].to_read_model() for row in res.all()]

    async def get_latest(
        self, session: AsyncSession, **filter_by
    ) -> Optional[BaseModel]:
        stmt = (
            select(self.model)
            .filter_by(**filter_by)
            .order_by(self.model.created_at.asc())
            .limit(1)
        )
        res = await session.execute(stmt)
        res = [row[0].to_read_model() for row in res.all()]
        if res:
            return res[0]
        return None
