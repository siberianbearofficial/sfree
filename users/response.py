from users.schema import UserCreateResponse, UserRead
from utils.response import ApiResponse


class PostUsersApiResponse(ApiResponse):
    data: UserCreateResponse


class GetUsersMeApiResponse(ApiResponse):
    data: UserRead
