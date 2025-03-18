import asyncio
import hashlib
import uuid
from datetime import datetime
from functools import lru_cache
from typing import AsyncGenerator, Optional

from loguru import logger
from sqlalchemy.ext.asyncio import AsyncSession

from buckets.repository import BucketRepository, get_bucket_repository
from buckets.schema import BucketRead
from gdrive.model import GDriveFileMetadataModel
from gdrive.repository import (
    GDriveRepository,
    GDriveFileMetadataRepository,
    get_gdrive_repository,
    get_gdrive_file_metadata_repository,
)
from gdrive.schema import GDriveRead, GDriveFileMetadataRead
from s3.model import FilePartModel, FileModel
from s3.repository import (
    FileRepository,
    FilePartRepository,
    get_file_repository,
    get_file_part_repository,
)
from s3.schema import FileRead, FilePartRead
from schemas import PutObjectResult, ListBucketResult, ListBucketResultContents
from sources.repository import SourceRepository, get_source_repository
from sources.schema import SourceRead, SourceType
from utils.exceptions import ExistsError, NotFoundError
from utils.google_drive_client import GoogleDriveClient
from utils.split_into_chunks import split_into_chunks
from utils.unitofwork import IUnitOfWork


class S3Service:
    def __init__(
        self,
        file_repository: FileRepository,
        file_part_repository: FilePartRepository,
        bucket_repository: BucketRepository,
        source_repository: SourceRepository,
        gdrive_repository: GDriveRepository,
        gdrive_file_metadata_repository: GDriveFileMetadataRepository,
    ):
        self._file_repository = file_repository
        self._file_part_repository = file_part_repository
        self._bucket_repository = bucket_repository
        self._source_repository = source_repository
        self._gdrive_repository = gdrive_repository
        self._gdrive_file_metadata_repository = gdrive_file_metadata_repository

    async def upload_file(
        self,
        uow: IUnitOfWork,
        bucket: BucketRead,
        filename: str,
        content: bytes,
    ) -> PutObjectResult:
        async with uow:
            file_with_this_name = await self._file_repository.get(
                uow.session, name=filename, bucket_key=bucket.key
            )
            if file_with_this_name:
                raise ExistsError(
                    "File with this name already exists in this bucket for this user."
                )

            sources: list[SourceRead] = await self._source_repository.get_all(
                uow.session, user_id=bucket.user_id
            )

            # todo тут нужна логика распределения частей файла по дискам, но пока сложим все на один гугл диск
            gdrive_source_id = None
            for source in sources:
                if source.type == SourceType.GDRIVE.value:
                    gdrive_source_id = source.id
                    break

            if not gdrive_source_id:
                raise ValueError(
                    "GDrive source not found for this user. Other source types are not supported yet."
                )

            gdrive: Optional[GDriveRead] = await self._gdrive_repository.get(
                uow.session, source_id=gdrive_source_id
            )
            if not gdrive:
                raise ValueError(
                    "GDrive source not found. Inconsistency in database. Consider asking for support."
                )

            logger.info(f"Source to upload: {source.name}")

            file_id = uuid.uuid4()
            file_created_at = datetime.now()
            file_model = FileModel(
                id=file_id,
                created_at=file_created_at,
                bucket_key=bucket.key,
                name=filename,
            )

            await self._file_repository.add(uow.session, file_model)
            await uow.commit()

            logger.info(f"File created: {file_model.name}")

        hash = hashlib.md5()
        number = 1
        for chunk in split_into_chunks(content):
            # todo разные чанки можно будет писать в разные места, поэтому клиент создается много раз
            async with GoogleDriveClient(key=gdrive.key) as client:
                await self.__upload_part_to_gdrive(
                    uow,
                    client,
                    file_id,
                    source.id,
                    number,
                    chunk,
                )
                logger.info(f"Uploaded part: {number}")
                number += 1
                hash.update(chunk)

        logger.success("Upload finished successfully")

        return PutObjectResult(ETag=hash.hexdigest())

    async def __upload_part_to_gdrive(
        self,
        uow: IUnitOfWork,
        client: GoogleDriveClient,
        file_id: uuid.UUID,
        source_id: uuid.UUID,
        number: int,
        data: bytes,
    ):
        created_at = datetime.now()
        gdrive_file_name = str(uuid.uuid4())

        gdrive_file_id = await client.upload_file_async(
            name=gdrive_file_name,
            directory_id="root",
            mimetype="application/octet-stream",
            data=data,
        )

        file_part_id = uuid.uuid4()
        hashed_data = hashlib.md5(data).hexdigest()

        file_part_model = FilePartModel(
            id=file_part_id,
            created_at=created_at,
            file_id=file_id,
            source_id=source_id,
            hash=hashed_data,
            number=number,
        )

        async with uow:
            await self._file_part_repository.add(uow.session, file_part_model)
            await uow.commit()

        gdrive_file_metadata_model = GDriveFileMetadataModel(
            created_at=created_at,
            file_part_id=file_part_id,
            gdrive_file_id=gdrive_file_id,
            gdrive_file_name=gdrive_file_name,
        )

        async with uow:
            await self._gdrive_file_metadata_repository.add(uow.session, gdrive_file_metadata_model)
            await uow.commit()

    async def get_files_by_bucket(
        self,
        uow: IUnitOfWork,
        bucket: BucketRead,
    ) -> ListBucketResult:
        async with uow:
            files = await self._file_repository.get_all(uow.session, bucket_key=bucket.key)
            return ListBucketResult(
                Name=bucket.key,
                Prefix="",
                KeyCount=len(files),
                MaxKeys=1000,
                IsTruncated=False,
                Delimiter="",
                EncodingType="url",
                ContinuationToken="",
                NextContinuationToken="",
                StartAfter="",
                Contents=[
                    ListBucketResultContents(
                        Key=file.name,
                        Size=100,
                        ETag="",
                        LastModified="1970-01-01T00:00:00Z",
                        StorageClass="STANDARD",
                    )
                    for file in files
                ],
                CommonPrefixes=None,
            )

    async def get_file_by_name(
        self,
        uow: IUnitOfWork,
        bucket: BucketRead,
        name: str,
    ) -> AsyncGenerator[bytes, None]:
        async with uow:
            file: Optional[FileRead] = await self._file_repository.get(
                uow.session, bucket_key=bucket.key, name=name
            )
            if not file:
                raise NotFoundError("File not found.")

            file_parts = await self.__get_file_parts(uow.session, file)
            print("File parts:", file_parts)

            return self.__download_file_parts(file_parts)

    async def __download_file_parts(self, file_parts: tuple[dict]) -> AsyncGenerator[bytes, None]:
        for part in file_parts:
            async with GoogleDriveClient(part.get("key", "")) as client:
                yield await client.download_file_async(part.get("gdrive_file_id", ""))

    async def __get_file_parts(self, session: AsyncSession, file: FileRead) -> tuple[dict]:
        file_parts: list[FilePartRead] = await self._file_part_repository.get_sorted_by_number(
            session, file_id=file.id
        )

        return await asyncio.gather(
            *map(
                lambda file_part: self.__get_file_part_metadata(session, file_part),
                file_parts,
            )
        )

    async def __get_file_part_metadata(
        self, session: AsyncSession, file_part: FilePartRead
    ) -> dict:
        source: Optional[SourceRead] = await self._source_repository.get(
            session, id=file_part.source_id
        )
        if not source:
            raise NotFoundError(
                f"Source {file_part.source_id} not found for file part {file_part.id}."
            )

        if source.type != SourceType.GDRIVE.value:
            raise ValueError("Only gdrive source type is supported.")

        gdrive = await self._gdrive_repository.get(session, source_id=source.id)
        if not gdrive:
            raise NotFoundError("Gdrive source not found.")

        metadata: Optional[GDriveFileMetadataRead] = (
            await self._gdrive_file_metadata_repository.get(session, file_part_id=file_part.id)
        )

        if not metadata:
            raise NotFoundError(f"File metadata not found for file part {file_part.id}.")

        return {
            "key": gdrive.key,
            "gdrive_file_id": metadata.gdrive_file_id,
            "gdrive_file_name": metadata.gdrive_file_name,
        }


@lru_cache
def get_s3_service() -> S3Service:
    return S3Service(
        file_repository=get_file_repository(),
        file_part_repository=get_file_part_repository(),
        bucket_repository=get_bucket_repository(),
        source_repository=get_source_repository(),
        gdrive_repository=get_gdrive_repository(),
        gdrive_file_metadata_repository=get_gdrive_file_metadata_repository(),
    )
