from sqlalchemy import ForeignKey, Column, Uuid, String, Integer

from buckets.model import BucketModel
from sources.model import SourceModel

from s3.schema import FileRead, FilePartRead

from utils.model import Model


class FileModel(Model):
    __tablename__ = "file"

    bucket_key = Column(String, ForeignKey(BucketModel.key), nullable=False, index=True)
    name = Column(String, nullable=False)

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

    file_id = Column(Uuid, ForeignKey(FileModel.id), nullable=False)
    source_id = Column(Uuid, ForeignKey(SourceModel.id), nullable=False)
    hash = Column(String, nullable=False)
    number = Column(Integer, nullable=False)

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
