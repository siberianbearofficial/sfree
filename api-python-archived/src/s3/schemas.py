from dataclasses import dataclass, field
from typing import Optional


@dataclass
class PutObjectResult:
    ETag: str = field(
        metadata={
            "examples": ["594cea4d7bf18899349805e4737bda6f"],
            "name": "ETag",
            "type": "Element",
        }
    )


@dataclass
class DeleteResultDeleted:
    Key: str = field(metadata={"examples": ["example.txt"], "name": "Key", "type": "Element"})


@dataclass
class DeleteResult:
    Deleted: DeleteResultDeleted


@dataclass
class ListBucketResultContents:
    ETag: str = field(
        metadata={
            "examples": ["594cea4d7bf18899349805e4737bda6f"],
            "name": "ETag",
            "type": "Element",
        }
    )
    Key: str = field(metadata={"examples": ["example.txt"], "name": "Key", "type": "Element"})
    LastModified: str = field(
        metadata={
            "examples": ["1970-01-01T00:00:00Z"],
            "name": "LastModified",
            "type": "Element",
        }
    )
    Size: int = field(metadata={"examples": [1024], "name": "Size", "type": "Element"})
    StorageClass: str = field(
        metadata={"examples": ["STANDARD"], "name": "StorageClass", "type": "Element"}
    )


@dataclass
class ListBucketResultCommonPrefixes:
    Prefix: str = field(metadata={"examples": ["example"], "name": "Prefix", "type": "Element"})


@dataclass
class ListBucketResult:
    IsTruncated: bool = field(
        metadata={"examples": [True], "name": "IsTruncated", "type": "Element"}
    )
    Contents: list[ListBucketResultContents]
    Name: str = field(metadata={"examples": ["test-bucket"], "name": "Name", "type": "Element"})
    Prefix: str = field(metadata={"examples": ["test-bucket"], "name": "Prefix", "type": "Element"})
    Delimiter: str = field(metadata={"examples": ["/"], "name": "Delimiter", "type": "Element"})
    MaxKeys: int = field(metadata={"examples": [1000], "name": "MaxKeys", "type": "Element"})
    CommonPrefixes: Optional[ListBucketResultCommonPrefixes]
    EncodingType: str = field(
        metadata={"examples": ["UTF-8"], "name": "EncodingType", "type": "Element"}
    )
    KeyCount: int = field(metadata={"examples": [1], "name": "KeyCount", "type": "Element"})
    ContinuationToken: str = field(
        metadata={"examples": ["0"], "name": "ContinuationToken", "type": "Element"}
    )
    NextContinuationToken: str = field(
        metadata={"examples": ["0"], "name": "NextContinuationToken", "type": "Element"}
    )
    StartAfter: str = field(
        metadata={"examples": ["example.txt"], "name": "StartAfter", "type": "Element"}
    )
