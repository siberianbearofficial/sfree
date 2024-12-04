from functools import lru_cache

from gdrive.model import GDriveModel, GDriveFileMetadataModel
from utils.repository import TimestampRepository


class GDriveRepository(TimestampRepository):
    model = GDriveModel


class GDriveFileMetadataRepository(TimestampRepository):
    model = GDriveFileMetadataModel


@lru_cache
def get_gdrive_repository() -> GDriveRepository:
    return GDriveRepository()


@lru_cache
def get_gdrive_file_metadata_repository() -> GDriveFileMetadataRepository:
    return GDriveFileMetadataRepository()
