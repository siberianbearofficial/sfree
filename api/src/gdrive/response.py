from gdrive.schema import GDriveCreateResponse
from utils.response import ApiResponse


class PostGDriveApiResponse(ApiResponse):
    data: GDriveCreateResponse
