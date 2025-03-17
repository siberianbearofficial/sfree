from functools import lru_cache

from src.sources.model import SourceModel
from src.utils.repository import TimestampRepository


class SourceRepository(TimestampRepository):
    model = SourceModel


@lru_cache
def get_source_repository() -> SourceRepository:
    return SourceRepository()
