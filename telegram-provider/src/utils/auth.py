from typing import Annotated

from fastapi import Depends
from fastapi.security import HTTPBasic, HTTPBasicCredentials
import bcrypt
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from ..utils.database import get_session
from ..users.models import User

security = HTTPBasic()

CredentialsDep = Annotated[HTTPBasicCredentials, Depends(security)]


async def get_current_user(
    credentials: CredentialsDep, session: Annotated[AsyncSession, Depends(get_session)]
) -> User:
    result = await session.execute(select(User).where(User.username == credentials.username))
    user = result.scalar_one_or_none()
    if not user:
        raise Exception("Unauthorized")
    if not bcrypt.checkpw(credentials.password.encode(), user.password_hash.encode()):
        raise Exception("Unauthorized")
    return user
