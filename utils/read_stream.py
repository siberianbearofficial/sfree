from typing import AsyncGenerator

from utils.config import CHUNK_SIZE


async def read_stream(stream: AsyncGenerator[bytes, None]) -> AsyncGenerator[bytes, None]:
    chunk = bytearray()

    async for chunk_part in stream:
        if len(chunk) + len(chunk_part) >= CHUNK_SIZE:
            slice_index = CHUNK_SIZE - len(chunk)

            chunk.extend(chunk_part[:slice_index])
            yield chunk

            chunk.clear()
            chunk.extend(chunk_part[slice_index:])
        else:
            chunk.extend(chunk_part)

    if len(chunk) > 0:
        yield chunk
