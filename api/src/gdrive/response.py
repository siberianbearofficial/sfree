from src.gdrive.schema import GDriveCreateResponse
from src.utils.response import ApiResponse


class PostGDriveApiResponse(ApiResponse):
    data: GDriveCreateResponse
