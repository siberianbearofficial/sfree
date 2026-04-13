from sqlalchemy import Uuid, ForeignKey, String
from sqlalchemy.orm import mapped_column, Mapped
from uuid import UUID

from src.gdrive.schema import GDriveRead, GDriveFileMetadataRead
from src.s3.model import FilePartModel
from src.sources.model import SourceModel

from src.utils.model import Model


class GDriveModel(Model):
    __tablename__ = "gdrive"

    source_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(SourceModel.id), nullable=False)
    key: Mapped[str] = mapped_column(String, nullable=False)

    def to_read_model(self):
        return GDriveRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            source_id=self.source_id,
            key=self.key,
        )


class GDriveFileMetadataModel(Model):
    __tablename__ = "gdrive_file_metadata"

    file_part_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(FilePartModel.id), nullable=False)
    gdrive_file_id: Mapped[str] = mapped_column(String, nullable=False)
    gdrive_file_name: Mapped[str] = mapped_column(String, nullable=False)

    def to_read_model(self) -> GDriveFileMetadataRead:
        return GDriveFileMetadataRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            file_part_id=self.file_part_id,
            gdrive_file_id=self.gdrive_file_id,
            gdrive_file_name=self.gdrive_file_name,
        )
