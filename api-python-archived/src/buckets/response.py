from src.buckets.schema import BucketCreateResponse
from src.utils.response import ApiResponse


class PostBucketsApiResponse(ApiResponse):
    data: BucketCreateResponse
