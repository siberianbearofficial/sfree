from fastapi import HTTPException
from loguru import logger


class NotFoundError(Exception):
    pass


class AuthenticationError(Exception):
    pass


class ExistsError(Exception):
    pass


def exception_handler(handler):
    async def wrapper(*args, **kwargs):
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

    # Fix signature of wrapper
    import inspect
    wrapper.__signature__ = inspect.Signature(
        parameters=[
            # Use all parameters from handler
            *inspect.signature(handler).parameters.values(),
            # Skip *args and **kwargs from wrapper parameters:
            *filter(
                lambda p: p.kind not in (inspect.Parameter.VAR_POSITIONAL, inspect.Parameter.VAR_KEYWORD),
                inspect.signature(wrapper).parameters.values()
            )
        ],
        return_annotation=inspect.signature(handler).return_annotation,
    )
    return wrapper
