from pydantic import BaseModel


class AccountCreate(BaseModel):
    phone: str


class CodeConfirm(BaseModel):
    code: str


class AccountOut(BaseModel):
    id: int
    phone: str
