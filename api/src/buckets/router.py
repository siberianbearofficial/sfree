from fastapi import APIRouter

from src.buckets.response import PostBucketsApiResponse
from src.buckets.schema import BucketCreate
from src.utils.basic_auth import UserDep
from src.utils.dependency import UOWDep, BucketServiceDep
from src.utils.exceptions import exception_handler

router = APIRouter()


@router.post(
    "",
    response_model=PostBucketsApiResponse,
    summary="Create a new bucket",
    description="Create a new bucket and generate S3 credentials for it",
)
@exception_handler
async def add_bucket(
    uow: UOWDep, bucket_service: BucketServiceDep, user: UserDep, bucket: BucketCreate
):
    created_bucket = await bucket_service.add_bucket(uow, bucket, user)
    return PostBucketsApiResponse(data=created_bucket, detail="Bucket was added.")
