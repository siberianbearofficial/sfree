from sqlalchemy import Column, Integer, String
from sqlalchemy.orm import relationship

from ..utils.models import Base


class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    username = Column(String, unique=True, nullable=False)
    password_hash = Column(String, nullable=False)
    accounts = relationship("Account", back_populates="user")
