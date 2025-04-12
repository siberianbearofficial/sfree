from typing import Optional

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from fastapi_xml import XmlAppResponse

from src.s3.schemas import (
    DeleteResultDeleted,
    DeleteResult,
    ListBucketResult,
    PutObjectResult,
)
from src.utils.dependency import S3ServiceDep, UOWDep
from src.utils.exceptions import AuthenticationError
from src.utils.s3_auth import S3AuthenticatedRequestDep

router = APIRouter()


@router.get(
    "/{bucket_key}",
    response_model=ListBucketResult,
    response_class=XmlAppResponse,
    summary="List objects",
    description="Get list of objects in the bucket (S3-compatible)",
)
async def list_objects(
    uow: UOWDep,
    s3_service: S3ServiceDep,
    request: S3AuthenticatedRequestDep,
    continuation_token: Optional[str] = None,
    delimiter: Optional[str] = None,
    encoding_type: str = "url",
    max_keys: int = 1000,
    prefix: str = "",
    start_after: Optional[str] = None,
    list_type: int = 2,
):
    """ListObjectsV2 S3 совместимый запрос."""

    if list_type != 2:
        raise NotImplementedError("Only ListObjectsV2 is supported.")

    try:
        pass
        # verify_signature(request, secret_key=SECRET_KEY)
    except ValueError as e:
        print(e)
        raise AuthenticationError from e

    files = await s3_service.get_files_by_bucket(uow, request.bucket)
    return XmlAppResponse(files)


@router.get(
    "/{bucket_key}/{name:path}",
    response_class=StreamingResponse,
    summary="Get object",
    description="Get file contents from the bucket (S3-compatible)",
)
async def get_object(
    uow: UOWDep, s3_service: S3ServiceDep, request: S3AuthenticatedRequestDep, name: str
):
    """GET Object S3 совместимый запрос."""

    # etag = hashlib.md5(file_path.read_bytes()).hexdigest()

    file_stream = await s3_service.get_file_by_name(uow, request.bucket, name)
    return StreamingResponse(
        file_stream,
        headers={
            "Content-Type": "application/octet-stream",
            "ETag": "",
        },
    )


@router.put(
    "/{bucket_key}/{name:path}",
    response_model=PutObjectResult,
    response_class=XmlAppResponse,
    summary="Put object",
    description="Put file contents to the bucket (S3-compatible)",
)
async def put_object(
    uow: UOWDep,
    s3_service: S3ServiceDep,
    request: S3AuthenticatedRequestDep,
    name: str,
):
    """PUT Object S3 совместимый запрос."""
    uploaded_file = await s3_service.upload_file(uow, request.bucket, name, request.content)
    return XmlAppResponse(uploaded_file)


@router.delete(
    "/{bucket_key}/{name:path}",
    response_model=DeleteResult,
    response_class=XmlAppResponse,
    summary="Delete object",
    description="Delete file from the bucket (S3-compatible)",
)
async def delete_object(
    _: S3AuthenticatedRequestDep,
    name: str,
):
    """DELETE Object S3 совместимый запрос."""

    deleted = DeleteResult(Deleted=DeleteResultDeleted(Key=name))
    return XmlAppResponse(deleted)
