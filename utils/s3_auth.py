import aws4
import hmac

from typing import Annotated
from fastapi import Depends, Path, Request
from loguru import logger

from buckets.schema import BucketRead
from utils.dependency import UOWDep, BucketServiceDep
from utils.exceptions import exception_handler, AuthenticationError


@exception_handler
async def get_bucket(
    uow: UOWDep,
    bucket_service: BucketServiceDep,
    bucket_key: Annotated[str, Path()],
    request: Request,
) -> BucketRead:
    # превращаем любую ошибку в AuthenticationError для безопасности
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
            raise RuntimeError("Empty access key")

        bucket = await bucket_service.get_bucket_by_access_key(
            uow, access_key=challenge.access_key_id
        )
        # сравниваем строки через hmac, чтобы по времени работы кода нельзя было понять, что произошло
        if not hmac.compare_digest(bucket.key, bucket_key):
            raise RuntimeError(f"Bucket key does not match: {bucket.key, bucket_key}")

        aws4.validate_challenge(challenge, bucket.access_secret)
    except Exception as e:
        # снаружи мы не поймем, что именно ломается, если будут баги, поэтому добавил лог внутри
        logger.error(e)
        raise AuthenticationError

    return BucketRead.model_validate(bucket)  # убираем креды


BucketDep = Annotated[BucketRead, Depends(get_bucket)]
