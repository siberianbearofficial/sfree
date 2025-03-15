from datetime import datetime

from sqlalchemy import Column, String, Uuid, ForeignKey, TIMESTAMP

from users.model import UserModel
from buckets.schema import BucketRead
from utils.database import Base


class BucketModel(Base):
    __tablename__ = "bucket"

    key = Column(String, primary_key=True)
    user_id = Column(Uuid, ForeignKey(UserModel.id), nullable=False, index=True)
    access_key = Column(String, nullable=False, unique=True, index=True)
    access_secret = Column(String, nullable=False)
    created_at = Column(
        TIMESTAMP,
        default=datetime.now,
        nullable=False,
    )
    updated_at = Column(
        TIMESTAMP,
        nullable=True,
    )
    deleted_at = Column(
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
