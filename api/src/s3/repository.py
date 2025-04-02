from functools import lru_cache
from typing import Sequence

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from src.s3.model import FilePartModel, FileModel
from src.utils.repository import TimestampRepository


class FileRepository(TimestampRepository):
    model = FileModel


class FilePartRepository(TimestampRepository):
    model = FilePartModel

    async def get_sorted_by_number(
        self, session: AsyncSession, **filter_by
    ) -> Sequence[FilePartModel]:
        stmt = select(self.model).filter_by(**filter_by).order_by(self.model.number.desc())

        res = await session.execute(stmt)
        return res.scalars().all()


@lru_cache
def get_file_repository() -> FileRepository:
    return FileRepository()


@lru_cache
def get_file_part_repository() -> FilePartRepository:
    return FilePartRepository()
