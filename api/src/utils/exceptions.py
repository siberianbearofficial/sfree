from fastapi import Request
from fastapi.responses import JSONResponse
from loguru import logger


class NotFoundError(Exception):
    pass


class AuthenticationError(Exception):
    pass


class ExistsError(Exception):
    pass


async def endpoints_exception_handler(request: Request, ex: Exception):
    if isinstance(ex, (ValueError, ExistsError)):
        logger.error(ex)
        return JSONResponse(status_code=400, content={"detail": str(ex)})
    elif isinstance(ex, AuthenticationError):
        logger.error(ex)
        return JSONResponse(status_code=401, content={"detail": str(ex)})
    elif isinstance(ex, PermissionError):
        logger.error(ex)
        return JSONResponse(status_code=403, content={"detail": str(ex)})
    elif isinstance(ex, NotFoundError):
        logger.error(ex)
        return JSONResponse(status_code=404, content={"detail": str(ex)})
    else:
        logger.exception(ex)
        return JSONResponse(status_code=500, content={"detail": str(ex)})
