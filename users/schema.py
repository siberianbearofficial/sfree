from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel


class UserWithHashedPassword(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    username: str
    hashed_password: str


class UserRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    username: str

    @staticmethod
    def from_with_hashed_password(
        with_hashed_password: UserWithHashedPassword,
    ) -> "UserRead":
        dump = with_hashed_password.model_dump(exclude={"hashed_password"})
        return UserRead.model_validate(dump)


class UserCreate(BaseModel):
    username: str  # todo validate username


class UserCreateResponse(BaseModel):
    id: UUID
    created_at: datetime
    password: str


class UserUpdate(BaseModel):
    username: Optional[str]  # todo validate username if not null
    password: Optional[str]  # todo validate password if not null
