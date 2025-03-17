from sqlalchemy import Column, String, Uuid, ForeignKey

from src.sources.schema import SourceRead
from src.users.model import UserModel

from src.utils.model import Model


class SourceModel(Model):
    __tablename__ = "source"

    type = Column(String, nullable=False)
    user_id = Column(Uuid, ForeignKey(UserModel.id), nullable=False, index=True)
    name = Column(String, nullable=False)

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
