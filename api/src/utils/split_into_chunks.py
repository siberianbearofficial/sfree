from typing import Generator

from utils.config import CHUNK_SIZE


def split_into_chunks(content: bytes, chunk_size: int = CHUNK_SIZE) -> Generator[bytes, None, None]:
    if chunk_size <= 0:
        raise ValueError(f"Invalid chunk size: {chunk_size}, must be greater than 0")

    for start in range(0, len(content), chunk_size):
        end = start + chunk_size
        if end >= len(content):
            yield content[start : len(content)]
            break
        else:
            yield content[start:end]
