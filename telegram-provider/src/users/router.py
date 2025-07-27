from fastapi import APIRouter, Depends
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
import bcrypt

from ..utils.database import get_session
from .models import User
from .schemas import UserCreate, UserOut

router = APIRouter(prefix="/users")


@router.post("", response_model=UserOut)
async def create_user(user: UserCreate, session: AsyncSession = Depends(get_session)):
    hash_pw = bcrypt.hashpw(user.password.encode(), bcrypt.gensalt()).decode()
    db_user = User(username=user.username, password_hash=hash_pw)
    session.add(db_user)
    await session.commit()
    await session.refresh(db_user)
    return UserOut(id=db_user.id, username=db_user.username)
