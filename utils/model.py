import uuid
from abc import abstractmethod
from datetime import datetime

from sqlalchemy import Column, Uuid, TIMESTAMP
from sqlalchemy.orm import declared_attr, DeclarativeMeta

from utils.database import Base


class BaseMixin(object):
    id = Column(Uuid, primary_key=True, default=uuid.uuid4)
    created_at = Column(
        TIMESTAMP,
        default=datetime.now,
        nullable=False,
    )
    updated_at = Column(
        TIMESTAMP,
        nullable=True,
    )
    deleted_at = Column(
        TIMESTAMP,
        nullable=True,
    )

    @declared_attr
    def __tablename__(cls):
        return cls.__name__.lower()


class Model(BaseMixin, Base, metaclass=DeclarativeMeta):
    __abstract__ = True

    @property
    @abstractmethod
    def __tablename__(self):
        """Table Name should be provided due to SQLAlchemy conventions."""

    @abstractmethod
    def to_read_model(self):
        """
        Converts the model to a Pydantic schema.
        """
        pass
