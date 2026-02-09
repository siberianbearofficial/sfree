import asyncio
from dataclasses import dataclass
from typing import Any

from aiobotocore.session import get_session
from aiohttp import BasicAuth, ClientSession, FormData


@dataclass(slots=True)
class E2EConfig:
    base_api_url: str
    gdrive_key: str

    @property
    def users_url(self) -> str:
        return f"{self.base_api_url}/api/v1/users"

    @property
    def buckets_url(self) -> str:
        return f"{self.base_api_url}/api/v1/buckets"

    @property
    def sources_url(self) -> str:
        return f"{self.base_api_url}/api/v1/sources"

    @property
    def gdrive_sources_url(self) -> str:
        return f"{self.sources_url}/gdrive"

    @property
    def s3_url(self) -> str:
        return f"{self.base_api_url}/api/s3"


class S3AASClient:
    def __init__(self, config: E2EConfig) -> None:
        self.config = config
        self._http = ClientSession(raise_for_status=True)

    async def close(self) -> None:
        await self._http.close()

    async def wait_ready(self, timeout_s: int = 60) -> None:
        deadline = asyncio.get_event_loop().time() + timeout_s
        last_err: Exception | None = None
        while asyncio.get_event_loop().time() < deadline:
            try:
                async with self._http.get(f"{self.config.base_api_url}/readyz") as response:
                    if response.status == 200:
                        return
            except Exception as exc:  # noqa: BLE001
                last_err = exc
            await asyncio.sleep(1)
        if last_err is not None:
            raise TimeoutError("API is not ready") from last_err
        raise TimeoutError("API is not ready")

    async def create_user(self, username: str) -> dict[str, Any]:
        async with self._http.post(self.config.users_url, json={"username": username}) as response:
            return await response.json()

    async def create_bucket(self, auth: BasicAuth, key: str) -> dict[str, Any]:
        async with self._http.post(self.config.buckets_url, auth=auth, json={"key": key}) as response:
            return await response.json()

    async def list_buckets(self, auth: BasicAuth) -> list[dict[str, Any]]:
        async with self._http.get(self.config.buckets_url, auth=auth) as response:
            return await response.json()

    async def delete_bucket(self, auth: BasicAuth, bucket_id: str) -> None:
        async with self._http.delete(f"{self.config.buckets_url}/{bucket_id}", auth=auth):
            return

    async def create_gdrive_source(self, auth: BasicAuth, name: str) -> dict[str, Any]:
        payload = {"name": name, "key": self.config.gdrive_key}
        async with self._http.post(self.config.gdrive_sources_url, auth=auth, json=payload) as response:
            return await response.json()

    async def list_sources(self, auth: BasicAuth) -> list[dict[str, Any]]:
        async with self._http.get(self.config.sources_url, auth=auth) as response:
            return await response.json()

    async def get_source_info(self, auth: BasicAuth, source_id: str) -> dict[str, Any]:
        async with self._http.get(f"{self.config.sources_url}/{source_id}/info", auth=auth) as response:
            return await response.json()

    async def delete_source(self, auth: BasicAuth, source_id: str) -> None:
        async with self._http.delete(f"{self.config.sources_url}/{source_id}", auth=auth):
            return

    async def upload_file_http(self, auth: BasicAuth, bucket_id: str, filename: str, content: bytes) -> dict[str, Any]:
        form = FormData()
        form.add_field("file", content, filename=filename, content_type="application/octet-stream")
        async with self._http.post(f"{self.config.buckets_url}/{bucket_id}/upload", auth=auth, data=form) as response:
            return await response.json()

    async def list_files(self, auth: BasicAuth, bucket_id: str) -> list[dict[str, Any]]:
        async with self._http.get(f"{self.config.buckets_url}/{bucket_id}/files", auth=auth) as response:
            return await response.json()

    async def download_file_http(self, auth: BasicAuth, bucket_id: str, file_id: str) -> bytes:
        async with self._http.get(f"{self.config.buckets_url}/{bucket_id}/files/{file_id}/download", auth=auth) as response:
            return await response.read()

    async def delete_file(self, auth: BasicAuth, bucket_id: str, file_id: str) -> None:
        async with self._http.delete(f"{self.config.buckets_url}/{bucket_id}/files/{file_id}", auth=auth):
            return

    async def download_file_s3(self, access_key: str, access_secret: str, bucket_key: str, object_key: str) -> bytes:
        session = get_session()
        async with session.create_client(
            "s3",
            region_name="us-east-1",
            endpoint_url=self.config.s3_url,
            aws_access_key_id=access_key,
            aws_secret_access_key=access_secret,
        ) as s3_client:
            response = await s3_client.get_object(Bucket=bucket_key, Key=object_key)
            return await response["Body"].read()
