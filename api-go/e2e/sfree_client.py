import asyncio
from dataclasses import dataclass
from typing import Any

from botocore.config import Config as BotocoreConfig
from aiobotocore.session import get_session
from aiohttp import BasicAuth, ClientSession, ClientTimeout, FormData


@dataclass(slots=True)
class E2EConfig:
    base_api_url: str
    source_type: str
    gdrive_key: str
    telegram_token: str
    telegram_chat_id: str
    s3_endpoint: str
    s3_bucket: str
    s3_access_key_id: str
    s3_secret_access_key: str
    s3_region: str
    s3_path_style: bool

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
    def telegram_sources_url(self) -> str:
        return f"{self.sources_url}/telegram"

    @property
    def s3_sources_url(self) -> str:
        return f"{self.sources_url}/s3"

    @property
    def s3_url(self) -> str:
        return f"{self.base_api_url}/api/s3"


class SFreeClient:
    def __init__(self, config: E2EConfig) -> None:
        self.config = config
        self._http = ClientSession(
            raise_for_status=True,
            timeout=ClientTimeout(total=120, sock_connect=10, sock_read=120),
        )

    async def close(self) -> None:
        await self._http.close()

    def _s3_client_kwargs(
        self,
        region: str,
        endpoint: str,
        access_key: str,
        access_secret: str,
    ) -> dict[str, Any]:
        return {
            "region_name": region,
            "endpoint_url": endpoint,
            "aws_access_key_id": access_key,
            "aws_secret_access_key": access_secret,
            "config": BotocoreConfig(
                connect_timeout=3,
                read_timeout=30,
                retries={"max_attempts": 3, "mode": "standard"},
            ),
        }

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


    async def ensure_s3_source_bucket(self, timeout_s: int = 60) -> None:
        session = get_session()
        deadline = asyncio.get_event_loop().time() + timeout_s
        while True:
            try:
                async with session.create_client(
                    "s3",
                    **self._s3_client_kwargs(
                        region=self.config.s3_region,
                        endpoint=self.config.s3_endpoint,
                        access_key=self.config.s3_access_key_id,
                        access_secret=self.config.s3_secret_access_key,
                    ),
                ) as s3_client:
                    try:
                        await s3_client.head_bucket(Bucket=self.config.s3_bucket)
                    except Exception:  # noqa: BLE001
                        create_payload: dict[str, Any] = {"Bucket": self.config.s3_bucket}
                        if self.config.s3_region != "us-east-1":
                            create_payload["CreateBucketConfiguration"] = {"LocationConstraint": self.config.s3_region}
                        await s3_client.create_bucket(**create_payload)
                    return
            except Exception:  # noqa: BLE001
                if asyncio.get_event_loop().time() >= deadline:
                    raise
                await asyncio.sleep(1)

    async def create_user(self, username: str) -> dict[str, Any]:
        async with self._http.post(self.config.users_url, json={"username": username}) as response:
            return await response.json()

    async def create_bucket(self, auth: BasicAuth, key: str, source_ids: list[str]) -> dict[str, Any]:
        payload = {"key": key, "source_ids": source_ids}
        async with self._http.post(self.config.buckets_url, auth=auth, json=payload) as response:
            return await response.json()

    async def list_buckets(self, auth: BasicAuth) -> list[dict[str, Any]]:
        async with self._http.get(self.config.buckets_url, auth=auth) as response:
            return await response.json()

    async def delete_bucket(self, auth: BasicAuth, bucket_id: str) -> None:
        async with self._http.delete(f"{self.config.buckets_url}/{bucket_id}", auth=auth):
            return

    async def create_source(self, auth: BasicAuth, name: str) -> dict[str, Any]:
        if self.config.source_type == "gdrive":
            return await self.create_gdrive_source(auth, name)
        if self.config.source_type == "telegram":
            return await self.create_telegram_source(auth, name)
        if self.config.source_type == "s3":
            return await self.create_s3_source(auth, name)
        raise ValueError(f"Unsupported source type: {self.config.source_type}")

    async def create_gdrive_source(self, auth: BasicAuth, name: str) -> dict[str, Any]:
        payload = {"name": name, "key": self.config.gdrive_key}
        async with self._http.post(self.config.gdrive_sources_url, auth=auth, json=payload) as response:
            return await response.json()

    async def create_telegram_source(self, auth: BasicAuth, name: str) -> dict[str, Any]:
        payload = {
            "name": name,
            "token": self.config.telegram_token,
            "chat_id": self.config.telegram_chat_id,
        }
        async with self._http.post(self.config.telegram_sources_url, auth=auth, json=payload) as response:
            return await response.json()


    async def create_s3_source(self, auth: BasicAuth, name: str) -> dict[str, Any]:
        payload = {
            "name": name,
            "endpoint": self.config.s3_endpoint,
            "bucket": self.config.s3_bucket,
            "access_key_id": self.config.s3_access_key_id,
            "secret_access_key": self.config.s3_secret_access_key,
            "region": self.config.s3_region,
            "path_style": self.config.s3_path_style,
        }
        async with self._http.post(self.config.s3_sources_url, auth=auth, json=payload) as response:
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

    async def delete_source_status(self, auth: BasicAuth, source_id: str) -> int:
        async with self._http.delete(
            f"{self.config.sources_url}/{source_id}",
            auth=auth,
            raise_for_status=False,
        ) as response:
            return response.status

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

    async def upload_file_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
        content: bytes,
        content_type: str | None = None,
        metadata: dict[str, str] | None = None,
    ) -> None:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            payload: dict[str, Any] = {"Bucket": bucket_key, "Key": object_key, "Body": content}
            if content_type is not None:
                payload["ContentType"] = content_type
            if metadata is not None:
                payload["Metadata"] = metadata
            await s3_client.put_object(**payload)

    async def generate_presigned_url_s3(
        self,
        access_key: str,
        access_secret: str,
        client_method: str,
        params: dict[str, Any],
        expires_in: int = 3600,
        http_method: str | None = None,
    ) -> str:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            payload: dict[str, Any] = {
                "ClientMethod": client_method,
                "Params": params,
                "ExpiresIn": expires_in,
            }
            if http_method is not None:
                payload["HttpMethod"] = http_method
            return await s3_client.generate_presigned_url(**payload)

    async def delete_object_s3(self, access_key: str, access_secret: str, bucket_key: str, object_key: str) -> None:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            await s3_client.delete_object(Bucket=bucket_key, Key=object_key)

    async def copy_object_s3(
        self,
        access_key: str,
        access_secret: str,
        source_bucket_key: str,
        source_object_key: str,
        dest_bucket_key: str,
        dest_object_key: str,
        metadata_directive: str | None = None,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            payload: dict[str, Any] = {
                "Bucket": dest_bucket_key,
                "Key": dest_object_key,
                "CopySource": {"Bucket": source_bucket_key, "Key": source_object_key},
            }
            if metadata_directive is not None:
                payload["MetadataDirective"] = metadata_directive
            return await s3_client.copy_object(**payload)

    async def download_file_s3(self, access_key: str, access_secret: str, bucket_key: str, object_key: str) -> bytes:
        response = await self.get_object_s3(
            access_key=access_key,
            access_secret=access_secret,
            bucket_key=bucket_key,
            object_key=object_key,
        )
        return response["Body"]

    async def get_object_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
        byte_range: str | None = None,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            payload: dict[str, Any] = {"Bucket": bucket_key, "Key": object_key}
            if byte_range is not None:
                payload["Range"] = byte_range
            response = await s3_client.get_object(**payload)
            body = await response["Body"].read()
            return {**response, "Body": body}

    async def head_object_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.head_object(Bucket=bucket_key, Key=object_key)

    async def list_objects_s3(self, access_key: str, access_secret: str, bucket_key: str) -> list[dict[str, Any]]:
        response = await self.list_objects_v2_s3(
            access_key=access_key,
            access_secret=access_secret,
            bucket_key=bucket_key,
        )
        return response.get("Contents", [])

    async def list_objects_v2_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        prefix: str | None = None,
        delimiter: str | None = None,
        max_keys: int | None = None,
        continuation_token: str | None = None,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            payload: dict[str, Any] = {"Bucket": bucket_key}
            if prefix is not None:
                payload["Prefix"] = prefix
            if delimiter is not None:
                payload["Delimiter"] = delimiter
            if max_keys is not None:
                payload["MaxKeys"] = max_keys
            if continuation_token is not None:
                payload["ContinuationToken"] = continuation_token
            return await s3_client.list_objects_v2(**payload)

    async def delete_objects_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_keys: list[str],
        quiet: bool = False,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.delete_objects(
                Bucket=bucket_key,
                Delete={
                    "Objects": [{"Key": object_key} for object_key in object_keys],
                    "Quiet": quiet,
                },
            )

    async def create_multipart_upload_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.create_multipart_upload(Bucket=bucket_key, Key=object_key)

    async def upload_part_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
        upload_id: str,
        part_number: int,
        content: bytes,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.upload_part(
                Bucket=bucket_key,
                Key=object_key,
                UploadId=upload_id,
                PartNumber=part_number,
                Body=content,
            )

    async def complete_multipart_upload_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
        upload_id: str,
        parts: list[dict[str, Any]],
    ) -> None:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            await s3_client.complete_multipart_upload(
                Bucket=bucket_key,
                Key=object_key,
                UploadId=upload_id,
                MultipartUpload={"Parts": parts},
            )

    async def list_multipart_uploads_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.list_multipart_uploads(Bucket=bucket_key)

    async def list_parts_s3(
        self,
        access_key: str,
        access_secret: str,
        bucket_key: str,
        object_key: str,
        upload_id: str,
    ) -> dict[str, Any]:
        session = get_session()
        async with session.create_client(
            "s3",
            **self._s3_client_kwargs(
                region="us-east-1",
                endpoint=self.config.s3_url,
                access_key=access_key,
                access_secret=access_secret,
            ),
        ) as s3_client:
            return await s3_client.list_parts(Bucket=bucket_key, Key=object_key, UploadId=upload_id)
