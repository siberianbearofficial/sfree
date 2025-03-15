from sqlalchemy import Column, String

from users.schema import UserRead, UserWithHashedPassword

from utils.model import Model


class UserModel(Model):
    __tablename__ = "user"

    username = Column(String, nullable=False)
    hashed_password = Column(String, nullable=False)

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
