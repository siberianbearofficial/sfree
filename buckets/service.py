from datetime import datetime
from functools import lru_cache
from typing import Optional

from buckets.exception import BucketExistsError
from buckets.model import BucketModel
from buckets.repository import BucketRepository, get_bucket_repository
from buckets.schema import BucketCreate, BucketCreateResponse, BucketRead, BucketReadWithCredentials
from users.schema import UserRead

from utils.exceptions import NotFoundError
from utils.password import generate_access_secret
from utils.unitofwork import IUnitOfWork


class BucketService:
    def __init__(self, bucket_repository: BucketRepository):
        self._bucket_repository = bucket_repository

    async def get_bucket(self, uow: IUnitOfWork, key: str) -> BucketRead:
        async with uow:
            bucket: Optional[BucketRead] = await self._bucket_repository.get(uow.session, key=key)
            if not bucket:
                raise NotFoundError("Bucket not found.")

            return bucket

    async def get_bucket_by_access_key(
        self, uow: IUnitOfWork, access_key: str
    ) -> BucketReadWithCredentials:
        async with uow:
            bucket: Optional[BucketModel] = await self._bucket_repository.get_model(
                uow.session, access_key=access_key
            )
            if not bucket:
                raise NotFoundError("Bucket not found.")

            return BucketReadWithCredentials.from_orm(bucket)

    async def add_bucket(
        self,
        uow: IUnitOfWork,
        bucket: BucketCreate,
        user: UserRead,
    ) -> BucketCreateResponse:
        created_at = datetime.now()
        access_key = f"{bucket.key}.{user.id}"
        access_secret = generate_access_secret()

        bucket_model = BucketModel(
            key=bucket.key,
            user_id=user.id,
            created_at=created_at,
            access_key=access_key,
            access_secret=access_secret,
        )

        async with uow:
            old_bucket = await self._bucket_repository.get(uow.session, key=bucket.key)
            if old_bucket:
                raise BucketExistsError

            await self._bucket_repository.add(uow.session, bucket_model)
            await uow.commit()

        return BucketCreateResponse(
            created_at=created_at,
            access_key=access_key,
            access_secret=access_secret,
        )


@lru_cache
def get_bucket_service() -> BucketService:
    return BucketService(bucket_repository=get_bucket_repository())
