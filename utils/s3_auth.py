import aws4
import hmac

from typing import Annotated
from fastapi import Depends, Path, Request
from pydantic import BaseModel

from buckets.schema import BucketRead
from utils.dependency import UOWDep, BucketServiceDep
from utils.exceptions import exception_handler, AuthenticationError, NotFoundError


class S3AuthenticatedRequest(BaseModel):
    content: bytes
    bucket: BucketRead


BucketKeyDep = Annotated[str, Path()]


@exception_handler
async def get_s3_authenticated_request(
    uow: UOWDep,
    bucket_service: BucketServiceDep,
    bucket_key: BucketKeyDep,
    request: Request,
) -> S3AuthenticatedRequest:
    try:
        payload = await request.body()

        challenge = aws4.generate_challenge(
            method=request.method,
            url=request.url,
            headers=request.headers,
            content=payload,
            supported_schemas=[aws4.AWSAuthSchema],
        )
        if not challenge.access_key_id:
            raise ValueError("Empty access key")

        bucket = await bucket_service.get_bucket_by_access_key(
            uow, access_key=challenge.access_key_id
        )
        # сравниваем строки через hmac, чтобы по времени работы кода нельзя было понять, что произошло
        if not hmac.compare_digest(bucket.key, bucket_key):
            raise ValueError(f"Bucket key does not match: {bucket.key, bucket_key}")

        aws4.validate_challenge(challenge, bucket.access_secret)
    except (aws4.AWS4Exception, ValueError, NotFoundError):
        # превращаем любую ошибку в AuthenticationError для безопасности
        raise AuthenticationError

    return S3AuthenticatedRequest(
        bucket=BucketRead.model_validate(bucket),  # убираем креды
        content=payload,
    )


S3AuthenticatedRequestDep = Annotated[S3AuthenticatedRequest, Depends(get_s3_authenticated_request)]
