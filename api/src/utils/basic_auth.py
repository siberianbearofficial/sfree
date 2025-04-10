from typing import Annotated

from fastapi import Depends
from fastapi.security import HTTPBasic, HTTPBasicCredentials

from src.users.schema import UserRead
from src.utils.dependency import UOWDep, UserServiceDep
from src.utils.exceptions import AuthenticationError, exception_handler

security = HTTPBasic(auto_error=True, description="User Authentication")

BasicAuthCredentialsDep = Annotated[HTTPBasicCredentials, Depends(security)]


# @exception_handler
async def get_user(
    uow: UOWDep, credentials: BasicAuthCredentialsDep, user_service: UserServiceDep
) -> UserRead:
    user = await user_service.get_user_by_username_and_password(
        uow, credentials.username, credentials.password
    )
    if not user:
        raise AuthenticationError("Not authenticated")

    return user


UserDep = Annotated[UserRead, Depends(get_user)]
