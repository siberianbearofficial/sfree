from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from telethon import TelegramClient
from telethon.sessions import StringSession
import os

from ..utils.database import get_session
from ..utils.auth import get_current_user
from .models import Account
from .schemas import AccountCreate, CodeConfirm, AccountOut

API_ID = int(os.getenv("TG_API_ID", "0"))
API_HASH = os.getenv("TG_API_HASH", "")

router = APIRouter(prefix="/accounts")


@router.post("", response_model=AccountOut)
async def start_login(
    data: AccountCreate,
    user=Depends(get_current_user),
    session: AsyncSession = Depends(get_session),
):
    if not API_ID or not API_HASH:
        raise HTTPException(status_code=500, detail="Telegram API credentials not set")
    client = TelegramClient(StringSession(), API_ID, API_HASH)
    await client.connect()
    sent = await client.send_code_request(data.phone)
    acc = Account(
        user_id=user.id,
        phone=data.phone,
        session=client.session.save(),
        phone_code_hash=sent.phone_code_hash,
    )
    session.add(acc)
    await session.commit()
    await session.refresh(acc)
    await client.disconnect()
    return AccountOut(id=acc.id, phone=acc.phone)


@router.post("/{account_id}/code", response_model=AccountOut)
async def finish_login(
    account_id: int,
    data: CodeConfirm,
    user=Depends(get_current_user),
    session: AsyncSession = Depends(get_session),
):
    result = await session.execute(
        select(Account).where(Account.id == account_id, Account.user_id == user.id)
    )
    acc = result.scalar_one_or_none()
    if not acc:
        raise HTTPException(status_code=404, detail="Account not found")
    client = TelegramClient(StringSession(acc.session), API_ID, API_HASH)
    await client.connect()
    await client.sign_in(acc.phone, data.code, phone_code_hash=acc.phone_code_hash)
    acc.session = client.session.save()
    acc.is_authorized = True
    acc.phone_code_hash = None
    await session.commit()
    await client.disconnect()
    return AccountOut(id=acc.id, phone=acc.phone)
