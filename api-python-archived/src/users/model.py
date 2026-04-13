from sqlalchemy import String
from sqlalchemy.orm import mapped_column, Mapped

from src.users.schema import UserRead, UserWithHashedPassword

from src.utils.model import Model


class UserModel(Model):
    __tablename__ = "user"

    username: Mapped[str] = mapped_column(String, nullable=False)
    hashed_password: Mapped[str] = mapped_column(String, nullable=False)

    def to_read_model(self) -> UserRead:
        return UserRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            username=self.username,
        )

    def to_with_hashed_password_model(self) -> UserWithHashedPassword:
        return UserWithHashedPassword(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            username=self.username,
            hashed_password=self.hashed_password,
        )
