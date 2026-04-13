from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel


class GDriveRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    source_id: UUID
    key: str


class GDriveSourceRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    user_id: UUID
    key: str
    name: str


class GDriveCreate(BaseModel):
    key: str
    name: str


class GDriveCreateResponse(BaseModel):
    id: UUID
    created_at: datetime


class GDriveFileMetadataRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    file_part_id: UUID
    gdrive_file_id: str
    gdrive_file_name: str
