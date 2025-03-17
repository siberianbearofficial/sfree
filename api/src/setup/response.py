from src.utils.response import ApiResponse


class PostMigrationsUpgradeResponse(ApiResponse):
    data: str
    detail: str = "Migration upgrade successful."


class GetPublicationReadyResponse(ApiResponse):
    data: str
    detail: str = "API is ready to be published."


class GetHealthResponse(ApiResponse):
    data: str
    detail: str = "API is healthy."


class GetReadyResponse(ApiResponse):
    data: str
    detail: str = "API is ready."


class GetRootResponse(ApiResponse):
    data: str = "S3aaS API"
    detail: str = "Visit /docs or /redoc for the full documentation."
