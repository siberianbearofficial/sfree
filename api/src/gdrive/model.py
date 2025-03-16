from sqlalchemy import Uuid, Column, ForeignKey, String

from src.gdrive.schema import GDriveRead, GDriveFileMetadataRead
from src.s3.model import FilePartModel
from src.sources.model import SourceModel

from src.utils.model import Model


class GDriveModel(Model):
    __tablename__ = "gdrive"

    source_id = Column(Uuid, ForeignKey(SourceModel.id), nullable=False)
    key = Column(String, nullable=False)

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

    file_part_id = Column(Uuid, ForeignKey(FilePartModel.id), nullable=False)
    gdrive_file_id = Column(String, nullable=False)
    gdrive_file_name = Column(String, nullable=False)

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
