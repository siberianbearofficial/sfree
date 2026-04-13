import asyncio
import io
import json

from oauth2client.service_account import ServiceAccountCredentials
from googleapiclient.discovery import build, Resource
from googleapiclient.errors import HttpError
from googleapiclient.http import MediaIoBaseDownload, MediaIoBaseUpload
from loguru import logger

from src.utils.google_drive_exceptions import (
    GoogleDriveNoServiceError,
    GoogleDriveDirectoryNotFound,
    GoogleDriveDirectoryCreationError,
    GoogleDriveFileNotFound,
    GoogleDriveFileDownloadError,
    GoogleDriveFileUploadError,
    GoogleDriveDirectoryDeletionError,
    GoogleDriveFileDeletionError,
    GoogleDriveFileUpdateError,
)


class GoogleDriveClient:
    _DIRECTORY_MIMETYPE = "application/vnd.google-apps.folder"
    _DRIVE_API_VERSION = "v3"

    def __init__(self, key: str):
        self.key: str = key
        self.credentials: ServiceAccountCredentials
        self.service: Resource

    def __enter__(self) -> "GoogleDriveClient":
        self.service = build(
            "drive",
            self._DRIVE_API_VERSION,
            credentials=self.key_to_credentials(self.key),
        )
        return self

    def __exit__(self, *args):
        self.service = None

    async def __aenter__(self) -> "GoogleDriveClient":
        self.service = build(
            "drive",
            self._DRIVE_API_VERSION,
            credentials=self.key_to_credentials(self.key),
        )
        return self

    async def __aexit__(self, *args):
        self.service = None

    @staticmethod
    def key_to_credentials(key: str) -> ServiceAccountCredentials:
        return ServiceAccountCredentials.from_json_keyfile_dict(json.loads(key))

    @staticmethod
    def __directory_to_dict(directory: dict, parent: str) -> dict:
        return {
            "directory_id": directory.get("id"),
            "name": directory.get("name"),
            "parent": parent,
        }

    @staticmethod
    def __file_to_dict(file: dict, directory_id: str) -> dict:
        return {
            "file_id": file.get("id"),
            "name": file.get("name"),
            "mimetype": file.get("mimeType"),
            "directory_id": directory_id,
        }

    def get_directories(self, parent: str = "root", page_size: int = 100) -> list:
        # todo желательно предоставить возможность получить не только первую страницу

        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            results = (
                self.service.files()
                .list(
                    q=f"'{parent}' in parents and mimeType = '{self._DIRECTORY_MIMETYPE}'",
                    spaces="drive",
                    fields="nextPageToken, files(id, name)",
                    pageSize=page_size,
                )
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveDirectoryNotFound(str(e))

        directories = results.get("files", [])
        return list(
            map(
                lambda directory: self.__directory_to_dict(directory, parent),
                directories,
            )
        )

    def add_directory(self, name: str, parent: str = "root"):
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            result = (
                self.service.files()
                .create(
                    body={
                        "name": name,
                        "parents": [parent],
                        "mimeType": self._DIRECTORY_MIMETYPE,
                    },
                    fields="id",
                )
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveDirectoryCreationError

        return result.get("id")

    def delete_directory(self, directory_id: str) -> str:
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            self.service.files().delete(fileId=directory_id).execute()
        except HttpError as e:
            if e.status_code == 404:
                raise GoogleDriveDirectoryNotFound
            raise GoogleDriveDirectoryDeletionError
        except Exception:
            raise GoogleDriveDirectoryDeletionError

        return directory_id

    def get_files(self, directory_id: str = "root", page_size: int = 100) -> list:
        # todo желательно предоставить возможность получить не только первую страницу

        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            results = (
                self.service.files()
                .list(
                    q=f"'{directory_id}' in parents and mimeType != '{self._DIRECTORY_MIMETYPE}'",
                    spaces="drive",
                    fields="nextPageToken, files(id, name, mimeType)",
                    pageSize=page_size,
                )
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveDirectoryNotFound

        files = results.get("files", [])
        return list(map(lambda file: self.__file_to_dict(file, directory_id), files))

    def get_file(self, file_id: str) -> dict:
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            file = (
                self.service.files()
                .get(fileId=file_id, fields="id, name, mimeType, parents")
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveFileNotFound

        return self.__file_to_dict(file, (file["parents"] or ["null"])[0])

    def download_file(self, file_id: str) -> bytes:
        if not self.service:
            raise GoogleDriveNoServiceError

        file = io.BytesIO()

        try:
            request = self.service.files().get_media(fileId=file_id)

            downloader = MediaIoBaseDownload(file, request)
            done = False
            while done is False:
                status, done = downloader.next_chunk()
        except Exception as e:
            logger.error(e)
            raise GoogleDriveFileDownloadError

        file.seek(0)

        return file.read()

    def upload_file(self, name: str, directory_id: str, mimetype: str, data: bytes) -> str:
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            file = (
                self.service.files()
                .create(
                    body={"name": name, "parents": [directory_id]},
                    media_body=MediaIoBaseUpload(io.BytesIO(data), mimetype=mimetype),
                    fields="id",
                )
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveFileUploadError

        return file.get("id")

    def update_file(
        self, file_id: str, name: str | None = None, directory_id: str | None = None
    ) -> str:
        if not self.service:
            raise GoogleDriveNoServiceError

        body = dict()
        if name is not None:
            body["name"] = name

        try:
            if directory_id is not None:
                req = self.service.files().update(
                    fileId=file_id,
                    body=body,
                    removeParents=self.get_file(file_id)["directory_id"],
                    addParents=directory_id,
                    fields="id",
                )
            else:
                req = self.service.files().update(fileId=file_id, body=body, fields="id")
            file = req.execute()
        except Exception as e:
            logger.error(e)
            raise GoogleDriveFileUpdateError

        return file.get("id")

    def update_file_data(self, file_id: str, mimetype: str, data: bytes) -> str:
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            file = (
                self.service.files()
                .update(
                    fileId=file_id,
                    media_body=MediaIoBaseUpload(io.BytesIO(data), mimetype=mimetype),
                    fields="id",
                )
                .execute()
            )
        except Exception as e:
            logger.error(e)
            raise GoogleDriveFileUpdateError

        return file.get("id")

    def delete_file(self, file_id: str) -> str:
        if not self.service:
            raise GoogleDriveNoServiceError

        try:
            self.service.files().delete(fileId=file_id).execute()
        except HttpError as e:
            if e.status_code == 404:
                raise GoogleDriveFileNotFound
            raise GoogleDriveFileDeletionError
        except Exception:
            raise GoogleDriveFileDeletionError

        return file_id

    async def get_directories_async(self, parent: str = "root", page_size: int = 100) -> list:
        return await asyncio.to_thread(self.get_directories, parent, page_size)

    async def add_directory_async(self, name: str, parent: str = "root") -> str:
        return await asyncio.to_thread(self.add_directory, name, parent)

    async def delete_directory_async(self, directory_id: str) -> str:
        return await asyncio.to_thread(self.delete_directory, directory_id)

    async def get_files_async(self, directory_id: str = "root", page_size: int = 100) -> list:
        return await asyncio.to_thread(self.get_files, directory_id, page_size)

    async def get_file_async(self, file_id: str) -> dict:
        return await asyncio.to_thread(self.get_file, file_id)

    async def download_file_async(self, file_id: str) -> bytes:
        return await asyncio.to_thread(self.download_file, file_id)

    async def upload_file_async(
        self, name: str, directory_id: str, mimetype: str, data: bytes
    ) -> str:
        return await asyncio.to_thread(self.upload_file, name, directory_id, mimetype, data)

    async def update_file_async(
        self, file_id: str, name: str | None = None, directory_id: str | None = None
    ) -> str:
        return await asyncio.to_thread(self.update_file, file_id, name, directory_id)

    async def update_file_data_async(self, file_id: str, mimetype: str, data: bytes) -> str:
        return await asyncio.to_thread(self.update_file_data, file_id, mimetype, data)

    async def delete_file_async(self, file_id: str) -> str:
        return await asyncio.to_thread(self.delete_file, file_id)
