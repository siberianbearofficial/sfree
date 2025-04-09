from fastapi import HTTPException
from loguru import logger

from typing import ParamSpec, TypeVar, Callable, Awaitable
from functools import wraps


class NotFoundError(Exception):
    pass


class AuthenticationError(Exception):
    pass


class ExistsError(Exception):
    pass


P = ParamSpec("P")
R = TypeVar("R")


def exception_handler(handler: Callable[P, Awaitable[R]]) -> Callable[P, Awaitable[R]]:
    """
    Перехватывает базовые Exception и превращает их в красивые HTTPException.
    """

    @wraps(handler)
    async def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        try:
            res = await handler(*args, **kwargs)
        except (ValueError, ExistsError) as ex:
            logger.error(ex)
            raise HTTPException(400, detail=str(ex))
        except AuthenticationError as ex:
            logger.error(ex)
            raise HTTPException(401, detail=str(ex))
        except PermissionError as ex:
            logger.error(ex)
            raise HTTPException(403, detail=str(ex))
        except NotFoundError as ex:
            logger.error(ex)
            raise HTTPException(404, detail=str(ex))
        except Exception as ex:
            logger.exception(ex)
            raise HTTPException(500, detail=str(ex))
        else:
            return res

    return wrapper
