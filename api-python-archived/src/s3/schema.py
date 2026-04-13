from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel


class FileRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    bucket_key: str
    name: str


class FilePartRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    file_id: UUID
    source_id: UUID
    hash: str
    number: int
