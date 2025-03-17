from sqlalchemy import String, Uuid, ForeignKey
from sqlalchemy.orm import mapped_column, Mapped
from uuid import UUID

from src.sources.schema import SourceRead
from src.users.model import UserModel

from src.utils.model import Model


class SourceModel(Model):
    __tablename__ = "source"

    type: Mapped[str] = mapped_column(String, nullable=False)
    user_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(UserModel.id), nullable=False, index=True)
    name: Mapped[str] = mapped_column(String, nullable=False)

    def to_read_model(self):
        return SourceRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            type=self.type,
            user_id=self.user_id,
            name=self.name,
        )
