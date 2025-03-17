from sqlalchemy import ForeignKey, Uuid, String, Integer
from sqlalchemy.orm import mapped_column, Mapped
from uuid import UUID

from src.buckets.model import BucketModel
from src.sources.model import SourceModel

from src.s3.schema import FileRead, FilePartRead

from src.utils.model import Model


class FileModel(Model):
    __tablename__ = "file"

    bucket_key: Mapped[str] = mapped_column(String, ForeignKey(BucketModel.key), nullable=False, index=True)
    name: Mapped[str] = mapped_column(String, nullable=False)

    def to_read_model(self):
        return FileRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            bucket_key=self.bucket_key,
            name=self.name,
        )


class FilePartModel(Model):
    __tablename__ = "file_part"

    file_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(FileModel.id), nullable=False)
    source_id: Mapped[UUID] = mapped_column(Uuid, ForeignKey(SourceModel.id), nullable=False)
    hash: Mapped[str] = mapped_column(String, nullable=False)
    number: Mapped[int] = mapped_column(Integer, nullable=False)

    def to_read_model(self):
        return FilePartRead(
            id=self.id,
            created_at=self.created_at,
            updated_at=self.updated_at,
            deleted_at=self.deleted_at,
            file_id=self.file_id,
            source_id=self.source_id,
            hash=self.hash,
            number=self.number,
        )
