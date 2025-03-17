from datetime import datetime

from sqlalchemy import String, Uuid, ForeignKey, TIMESTAMP
from sqlalchemy.orm import Mapped, mapped_column
from uuid import UUID

from src.users.model import UserModel
from src.buckets.schema import BucketRead
from src.utils.database import Base


class BucketModel(Base):
    __tablename__ = "bucket"

    key: Mapped[str] = mapped_column(String, primary_key=True)
    user_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(UserModel.id), nullable=False, index=True)
    access_key: Mapped[str] = mapped_column(String, nullable=False)
    access_secret: Mapped[str] = mapped_column(String, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        TIMESTAMP,
        default=datetime.now,
        nullable=False,
    )
    updated_at: Mapped[datetime] = mapped_column(
        TIMESTAMP,
        nullable=True,
    )
    deleted_at: Mapped[datetime] = mapped_column(
        TIMESTAMP,
        nullable=True,
    )

    def to_read_model(self):
        return BucketRead(
            key=self.key,
            user_id=self.user_id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
        )
