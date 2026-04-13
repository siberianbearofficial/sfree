from pydantic import BaseModel


class ApiResponse(BaseModel):
    detail: str
