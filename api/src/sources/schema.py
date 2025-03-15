from datetime import datetime
from enum import Enum
from typing import Optional
from uuid import UUID

from pydantic import BaseModel


class SourceRead(BaseModel):
    id: UUID
    created_at: datetime
    updated_at: Optional[datetime]
    deleted_at: Optional[datetime]
    type: str
    user_id: UUID
    name: str


class SourceType(Enum):
    GDRIVE = "gdrive"
