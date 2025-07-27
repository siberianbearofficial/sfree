import asyncio
from fastapi import FastAPI
from sqlalchemy.ext.asyncio import AsyncSession

from src.utils.database import get_engine, get_session
from src.utils.models import Base
from src.users.router import router as users_router
from src.accounts.router import router as accounts_router
from src.dialogs.router import router as dialogs_router
from src.messages.router import router as messages_router

app = FastAPI(title="Telegram Provider")
app.include_router(users_router, tags=["users"])
app.include_router(accounts_router, tags=["accounts"])
app.include_router(dialogs_router, tags=["dialogs"])
app.include_router(messages_router, tags=["messages"])


@app.on_event("startup")
async def startup():
    engine = get_engine()
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)


@app.get("/readyz")
async def readyz():
    return {"data": "ready"}


@app.get("/healthz")
async def healthz():
    try:
        async for _ in get_session():
            break
        return {"data": "healthy"}
    except Exception as e:
        return {"data": "error", "detail": str(e)}
