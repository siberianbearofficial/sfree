from sqlalchemy import Column, Integer, String, ForeignKey, Boolean
from sqlalchemy.orm import relationship

from ..utils.models import Base


class Account(Base):
    __tablename__ = "accounts"
    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    phone = Column(String, nullable=False)
    session = Column(String, nullable=True)
    phone_code_hash = Column(String, nullable=True)
    is_authorized = Column(Boolean, default=False)
    user = relationship("User", back_populates="accounts")
