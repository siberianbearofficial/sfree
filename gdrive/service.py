import uuid
from datetime import datetime

from gdrive.model import GDriveModel
from gdrive.repository import GDriveRepository, get_gdrive_repository
from gdrive.schema import GDriveCreate, GDriveCreateResponse
from sources.model import SourceModel
from sources.repository import SourceRepository, get_source_repository
from sources.schema import SourceType
from users.schema import UserRead
from utils.unitofwork import IUnitOfWork


class GDriveService:
    def __init__(
        self, gdrive_repository: GDriveRepository, source_repository: SourceRepository
    ):
        self._gdrive_repository = gdrive_repository
        self._source_repository = source_repository

    async def add_gdrive(
        self, uow: IUnitOfWork, gdrive: GDriveCreate, user: UserRead
    ) -> GDriveCreateResponse:
        created_at = datetime.now()
        source_id = uuid.uuid4()

        source_model = SourceModel(
            id=source_id,
            created_at=created_at,
            type=SourceType.GDRIVE.value,
            name=gdrive.name,
            user_id=user.id,
        )

        async with uow:
            await self._source_repository.add(uow.session, source_model)
            await uow.commit()

        gdrive_id = uuid.uuid4()

        gdrive_model = GDriveModel(
            id=gdrive_id,
            created_at=created_at,
            source_id=source_id,
            key=gdrive.key,
        )

        async with uow:
            await self._gdrive_repository.add(uow.session, gdrive_model)
            await uow.commit()

        return GDriveCreateResponse(
            id=gdrive_id,
            created_at=created_at,
        )


def get_gdrive_service() -> GDriveService:
    return GDriveService(
        gdrive_repository=get_gdrive_repository(),
        source_repository=get_source_repository(),
    )
