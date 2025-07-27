from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from telethon import TelegramClient
from telethon.sessions import StringSession

from ..utils.database import get_session
from ..utils.auth import get_current_user
from ..accounts.models import Account

router = APIRouter(prefix="/messages")


@router.get("/{account_id}/{dialog_id}")
async def list_messages(
    account_id: int,
    dialog_id: int,
    limit: int = Query(20, gt=0),
    offset: int = Query(0, ge=0),
    user=Depends(get_current_user),
    session: AsyncSession = Depends(get_session),
):
    result = await session.execute(
        select(Account).where(Account.id == account_id, Account.user_id == user.id)
    )
    acc = result.scalar_one_or_none()
    if not acc or not acc.is_authorized:
        raise HTTPException(status_code=404, detail="Account not found")
    client = TelegramClient(StringSession(acc.session), 0, "")
    await client.connect()
    entity = await client.get_entity(dialog_id)
    total = []
    async for msg in client.iter_messages(entity, limit=limit, offset_id=offset):
        sender = await msg.get_sender()
        sender_name = sender.first_name or sender.username or ""
        sender_type = "me" if sender.is_self else "interlocutor"
        total.append(
            {
                "sentAt": msg.date.isoformat(),
                "sender": sender_name,
                "senderType": sender_type,
                "content": msg.message or "",
            }
        )
    await client.disconnect()
    return list(reversed(total))


@router.get("/{account_id}/{dialog_id}/all")
async def list_all_messages(
    account_id: int,
    dialog_id: int,
    user=Depends(get_current_user),
    session: AsyncSession = Depends(get_session),
):
    return await list_messages(
        account_id, dialog_id, limit=10000, offset=0, user=user, session=session
    )


@router.post("/{account_id}/{dialog_id}")
async def send_message(
    account_id: int,
    dialog_id: int,
    text: str,
    user=Depends(get_current_user),
    session: AsyncSession = Depends(get_session),
):
    result = await session.execute(
        select(Account).where(Account.id == account_id, Account.user_id == user.id)
    )
    acc = result.scalar_one_or_none()
    if not acc or not acc.is_authorized:
        raise HTTPException(status_code=404, detail="Account not found")
    client = TelegramClient(StringSession(acc.session), 0, "")
    await client.connect()
    await client.send_message(dialog_id, text)
    await client.disconnect()
    return {"status": "sent"}
