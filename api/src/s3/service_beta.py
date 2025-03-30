from src.s3.service import S3Service

import hashlib
import uuid
from queue import Queue, Empty

from datetime import datetime
from typing import Optional, Dict, Type
from loguru import logger

from src.buckets.schema import BucketRead
from src.gdrive.schema import BaseSourceModel
from src.s3.model import FileModel
from src.s3.schemas import PutObjectResult
from src.sources.schema import SourceRead, SourceType
from src.utils.exceptions import ExistsError, NoAvailableSourceError
from src.utils.google_drive_client import GoogleDriveClient
from src.utils.split_into_chunks import split_into_chunks
from src.utils.unitofwork import IUnitOfWork
from src.s3.repository import (
    FileRepository,
    FilePartRepository,
)
from src.buckets.repository import BucketRepository
from src.sources.repository import SourceRepository
from src.gdrive.repository import GDriveRepository, GDriveFileMetadataRepository
from src.utils.repository import TimestampRepository


class S3ServiceBeta(S3Service):
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
        self._repositories: Dict[str, TimestampRepository]
        self._clients: Dict[
            str, GoogleDriveClient
        ]  # TODO: заменить GoogleDriveClient на абстрактный класс AbsClient
        self.register_source(SourceType.GDRIVE.value, self._gdrive_repository, GoogleDriveClient)

    def register_source(
        self, type: str, repo: TimestampRepository, client: Type[GoogleDriveClient]
    ) -> None:
        if not isinstance(repo, TimestampRepository):
            raise ValueError("repo must be instance of TimestampRepository")
        if not issubclass(client, GoogleDriveClient):
            raise ValueError("client must be subclass of AbsClient")
        self._repositories[type] = repo
        self._clients[type] = client

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
            source_distr = SourceDistributor(
                sources=sources, uow=uow, repositories=self._repositories
            )
            for chunk in split_into_chunks(content):
                # надо будет замапить сурсы на соответствующие репозитории и возвращать уже GDriveRead и тд
                current_source = source_distr.next()
                source_model: Optional[BaseSourceModel] = await self._repositories[
                    current_source.type
                ].get(uow.session, source_id=current_source.id)
                if source_model is None:
                    uow.rollback()  # нужно ли делать так?
                    raise ValueError(
                        "GDrive source not found. Inconsistency in database. Consider asking for support."
                    )
                # todo разные чанки можно будет писать в разные места, поэтому клиент создается много раз
                async with self._clients[current_source.type](key=source_model.key) as client:
                    await self.__upload_part_to_gdrive(
                        uow,
                        client,
                        file_id,
                        current_source.id,
                        number,
                        chunk,
                    )
                    logger.info(f"Uploaded part: {number}")
                    number += 1
                    hash.update(chunk)

            logger.success("Upload finished successfully")

        return PutObjectResult(ETag=hash.hexdigest())


class SourceDistributor:
    def __init__(self, sources: list[SourceRead], uow: IUnitOfWork, file: FileModel | None = None):
        self._supported_types = [SourceType.GDRIVE.value]
        self._sources = sources
        self._uow = uow
        self._queue = None

        if file is not None and not self.check_space():
            raise ValueError("file can't be uploaded")  # ошибку другую конечно

        self.init_queue()

    def check_space(self) -> bool:
        # сделать логику проверки доступности необходимого пространства на дисках в зависимости от переданного FileModel
        return True

    def is_source_available(self, source: SourceRead, data_size: int) -> bool:
        # проверяет есть ли доступное место на конкретном сурсе
        return True

    def init_queue(self) -> None:
        self._queue = Queue()
        for source in self._sources:
            if source.type in self._supported_types:
                self._queue.put(source)

        if self._queue.empty():
            raise ValueError("Supported sources not found for this user.")

    def next(self) -> SourceRead:
        try:
            source = self._queue.get()
        except Empty:
            raise NoAvailableSourceError("No available source with enought space for data")

        if self.is_source_available(source=source, data_size=0):
            self._queue.put(source)
            return source
        else:
            return self.next()
