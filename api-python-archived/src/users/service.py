import uuid
from datetime import datetime
from typing import Optional

from src.users.model import UserModel
from src.users.repository import UserRepository, get_user_repository
from src.users.schema import UserCreate, UserCreateResponse, UserRead
from src.utils.password import generate_password, hash_password, check_password
from src.utils.unitofwork import IUnitOfWork


class UserService:
    def __init__(self, user_repository: UserRepository):
        self._user_repository = user_repository

    async def add_user(self, uow: IUnitOfWork, user: UserCreate) -> UserCreateResponse:
        id = uuid.uuid4()
        created_at = datetime.now()
        password = generate_password()
        hashed_password = hash_password(password)

        user_model = UserModel(
            id=id,
            created_at=created_at,
            username=user.username,
            hashed_password=hashed_password,
        )

        async with uow:
            await self._user_repository.add(uow.session, user_model)
            await uow.commit()

        return UserCreateResponse(
            id=id,
            created_at=created_at,
            password=password,
        )

    async def get_user_by_username_and_password(
        self, uow: IUnitOfWork, username: str, password: str
    ) -> Optional[UserRead]:
        async with uow:
            user = await self._user_repository.get_with_hashed_password(
                uow.session, username=username
            )
            if not user:
                return None

            if not check_password(password, user.hashed_password):
                return None

            return UserRead.from_with_hashed_password(user)


def get_user_service():
    return UserService(user_repository=get_user_repository())
