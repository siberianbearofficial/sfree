from src.utils.exceptions import ExistsError


class BucketExistsError(ExistsError):
    def __str__(self):
        return "Bucket with specified key already exists."
