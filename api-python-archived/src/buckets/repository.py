from functools import lru_cache

from sqlalchemy.ext.asyncio import AsyncSession

from src.buckets.model import BucketModel

from src.utils.repository import TimestampRepository


class BucketRepository(TimestampRepository):
    model = BucketModel

    async def add(self, session: AsyncSession, data: BucketModel) -> None:
        session.add(data)
        await session.flush()


@lru_cache
def get_bucket_repository() -> BucketRepository:
    return BucketRepository()
