import pytest
import aws4
import hmac
import uuid

from datetime import datetime
from unittest.mock import MagicMock, AsyncMock, patch

from src.buckets.schema import BucketRead, BucketReadWithCredentials
from src.utils.exceptions import NotFoundError, AuthenticationError
from src.utils.s3_auth import get_s3_authenticated_request, S3AuthenticatedRequest


@pytest.mark.asyncio
async def test_success():
    request = AsyncMock()
    request.method = "PUT"
    request.url = "http://localhost:8000/api/v1/s3/test-bucket/example.txt"
    request.headers = {}
    request.body.return_value = b"payload"

    challenge = MagicMock(access_key_id="access_key")

    bucket = BucketReadWithCredentials(
        id=uuid.uuid4(),
        key="test-bucket",
        user_id=uuid.uuid4(),
        access_key="access_key",
        access_secret="secret",
        created_at=datetime.now(),
        updated_at=None,
        deleted_at=None,
    )

    bucket_svc = AsyncMock()
    bucket_svc.get_bucket_by_access_key.return_value = bucket

    uow = AsyncMock()

    with (
        patch("aws4.generate_challenge", return_value=challenge),
        patch("hmac.compare_digest", return_value=True),
        patch("aws4.validate_challenge", return_value=None),
    ):
        result = await get_s3_authenticated_request(
            uow=uow, bucket_service=bucket_svc, bucket_key="test-bucket", request=request
        )

        assert isinstance(result, S3AuthenticatedRequest)
        assert result.bucket == BucketRead.model_validate(bucket)
        assert result.content == b"payload"

        aws4.generate_challenge.assert_called_once_with(  # type: ignore[attr-defined]
            method=request.method,
            url=request.url,
            headers=request.headers,
            content=b"payload",
            supported_schemas=[aws4.AWSAuthSchema],
        )
        bucket_svc.get_bucket_by_access_key.assert_awaited_once_with(uow, access_key="access_key")
        hmac.compare_digest.assert_called_once_with("test-bucket", "test-bucket")  # type: ignore[attr-defined]
        aws4.validate_challenge.assert_called_once_with(challenge, "secret")  # type: ignore[attr-defined]


@pytest.mark.asyncio
async def test_empty_access_key():
    with (
        patch("aws4.generate_challenge", return_value=MagicMock(access_key_id=None)),
        pytest.raises(AuthenticationError),
    ):
        await get_s3_authenticated_request(AsyncMock(), AsyncMock(), "key", AsyncMock())


@pytest.mark.asyncio
async def test_bucket_key_mismatch():
    bucket_svc = AsyncMock()
    bucket_svc.get_bucket_by_access_key.return_value = MagicMock(key="real_key")

    with (
        patch("aws4.generate_challenge", return_value=MagicMock(access_key_id="access_key")),
        patch("hmac.compare_digest", return_value=False),
        pytest.raises(AuthenticationError),
    ):
        await get_s3_authenticated_request(AsyncMock(), bucket_svc, "key", AsyncMock())


@pytest.mark.asyncio
async def test_validation_error():
    bucket_svc = AsyncMock()
    bucket_svc.get_bucket_by_access_key.return_value = MagicMock(key="key", access_secret="secret")

    with (
        patch("aws4.generate_challenge", return_value=MagicMock(access_key_id="access_key")),
        patch("hmac.compare_digest", return_value=True),
        patch("aws4.validate_challenge", side_effect=aws4.AWS4Exception),
        pytest.raises(AuthenticationError),
    ):
        await get_s3_authenticated_request(AsyncMock(), bucket_svc, "key", AsyncMock())


@pytest.mark.asyncio
async def test_get_bucket_error():
    bucket_svc = AsyncMock()
    bucket_svc.get_bucket_by_access_key.side_effect = NotFoundError

    with (
        patch("aws4.generate_challenge", return_value=MagicMock(access_key_id="access_key")),
        pytest.raises(AuthenticationError),
    ):
        await get_s3_authenticated_request(AsyncMock(), bucket_svc, "key", AsyncMock())
