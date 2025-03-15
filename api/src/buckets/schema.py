from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel, ConfigDict


class BucketRead(BaseModel):
    key: str
    user_id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]

    model_config = ConfigDict(from_attributes=True)


class BucketCredentials(BaseModel):
    access_key: str
    access_secret: str


class BucketReadWithCredentials(BucketRead, BucketCredentials):
    pass


class BucketCreate(BaseModel):
    key: str  # todo validate key


class BucketCreateResponse(BucketCredentials):
    created_at: datetime


class BucketUpdate(BaseModel):
    key: str  # todo validate key
