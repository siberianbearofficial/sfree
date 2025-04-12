from fastapi import APIRouter

from src.gdrive.response import PostGDriveApiResponse
from src.gdrive.schema import GDriveCreate
from src.utils.basic_auth import UserDep
from src.utils.dependency import UOWDep, GDriveServiceDep

router = APIRouter()


@router.post(
    "",
    response_model=PostGDriveApiResponse,
    summary="Create a new Google Drive source",
    description="Create a new Google Drive source with service account authorization by provided json key",
)
async def add_gdrive(
    uow: UOWDep, gdrive_service: GDriveServiceDep, user: UserDep, gdrive: GDriveCreate
):
    created_gdrive = await gdrive_service.add_gdrive(uow, gdrive, user)
    return PostGDriveApiResponse(data=created_gdrive, detail="Google Drive source was added.")
