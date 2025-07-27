from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from telethon import TelegramClient
from telethon.sessions import StringSession

from ..utils.database import get_session
from ..utils.auth import get_current_user
from ..accounts.models import Account

router = APIRouter(prefix="/dialogs")


@router.get("/{account_id}")
async def list_dialogs(
    account_id: int, user=Depends(get_current_user), session: AsyncSession = Depends(get_session)
):
    result = await session.execute(
        select(Account).where(Account.id == account_id, Account.user_id == user.id)
    )
    acc = result.scalar_one_or_none()
    if not acc or not acc.is_authorized:
        raise HTTPException(status_code=404, detail="Account not found")
    client = TelegramClient(StringSession(acc.session), 0, "")
    await client.connect()
    me_entity = await client.get_me()
    me_name = me_entity.first_name or me_entity.username or "me"
    dialogs = []
    async for dialog in client.iter_dialogs():
        interlocutors = []
        dtype = "dialog"
        if dialog.is_group:
            dtype = "group"
            participants = await client.get_participants(dialog)
            interlocutors = [p.first_name or p.username or "" for p in participants if not p.bot]
        elif dialog.is_channel:
            dtype = "channel"
            participants = []
            interlocutors = []
        else:
            interlocutors.append(dialog.entity.first_name or dialog.entity.username or "")
        dialogs.append(
            {
                "id": dialog.id,
                "title": dialog.name,
                "me": me_name,
                "interlocutors": interlocutors,
                "type": dtype,
            }
        )
    await client.disconnect()
    return dialogs
