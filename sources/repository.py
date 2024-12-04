from functools import lru_cache

from sources.model import SourceModel
from utils.repository import TimestampRepository


class SourceRepository(TimestampRepository):
    model = SourceModel


@lru_cache
def get_source_repository() -> SourceRepository:
    return SourceRepository()
