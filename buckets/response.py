from buckets.schema import BucketCreateResponse
from utils.response import ApiResponse


class PostBucketsApiResponse(ApiResponse):
    data: BucketCreateResponse
