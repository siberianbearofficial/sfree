from functools import lru_cache
from typing import Optional

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from src.users.model import UserModel
from src.users.schema import UserWithHashedPassword

from src.utils.repository import TimestampRepository


class UserRepository(TimestampRepository):
    model = UserModel

    async def get_with_hashed_password(
        self, session: AsyncSession, **filter_by
    ) -> Optional[UserWithHashedPassword]:
        stmt = select(self.model).filter_by(**filter_by).limit(1)
        res = await session.execute(stmt)
        res = [row[0].to_with_hashed_password_model() for row in res.all()]
        if res:
            return res[0]
        return None


@lru_cache
def get_user_repository():
    return UserRepository()
