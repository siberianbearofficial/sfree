from src.users.schema import UserCreateResponse, UserRead
from src.utils.response import ApiResponse


class PostUsersApiResponse(ApiResponse):
    data: UserCreateResponse


class GetUsersMeApiResponse(ApiResponse):
    data: UserRead
