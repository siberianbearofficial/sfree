from functools import lru_cache

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from s3.model import FilePartModel, FileModel
from s3.schema import FilePartRead
from utils.repository import TimestampRepository


class FileRepository(TimestampRepository):
    model = FileModel


class FilePartRepository(TimestampRepository):
    model = FilePartModel

    async def get_sorted_by_number(
        self, session: AsyncSession, **filter_by
    ) -> list[FilePartRead]:
        stmt = (
            select(self.model).filter_by(**filter_by).order_by(self.model.number.desc())
        )

        res = await session.execute(stmt)
        return [row[0].to_read_model() for row in res.all()]


@lru_cache
def get_file_repository() -> FileRepository:
    return FileRepository()


@lru_cache
def get_file_part_repository() -> FilePartRepository:
    return FilePartRepository()
