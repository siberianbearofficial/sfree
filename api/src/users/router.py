from fastapi import APIRouter

from src.users.response import PostUsersApiResponse, GetUsersMeApiResponse
from src.users.schema import UserCreate
from src.utils.basic_auth import UserDep
from src.utils.dependency import UOWDep, UserServiceDep
from src.utils.exceptions import exception_handler

router = APIRouter()


@router.post(
    "",
    response_model=PostUsersApiResponse,
    summary="Create a new user",
    description="Create a new user with generated password",
)
@exception_handler
async def add_user(uow: UOWDep, user_service: UserServiceDep, user: UserCreate):
    created_user = await user_service.add_user(uow, user)
    return PostUsersApiResponse(data=created_user, detail="User was added.")


@router.get(
    "/me",
    response_model=GetUsersMeApiResponse,
    summary="Get authenticated user",
    description="Validate credentials and get user details",
)
@exception_handler
async def get_me(user: UserDep):
    return GetUsersMeApiResponse(data=user, detail="User was selected.")
