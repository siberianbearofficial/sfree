from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel


class BucketRead(BaseModel):
    key: str
    user_id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]


class BucketCreate(BaseModel):
    key: str  # todo validate key


class BucketCreateResponse(BaseModel):
    created_at: datetime
    access_key: str
    access_secret: str


class BucketUpdate(BaseModel):
    key: str  # todo validate key
