import secrets
import string
import bcrypt

from utils.config import MIN_PASSWORD_LENGTH, ACCESS_SECRET_LENGTH

ALPHABET = string.ascii_letters + string.digits + string.punctuation


def generate_password() -> str:
    return "".join(secrets.choice(ALPHABET) for _ in range(MIN_PASSWORD_LENGTH))


def hash_password(password: str) -> str:
    return bcrypt.hashpw(password.encode(), bcrypt.gensalt()).decode()


def check_password(password: str, hashed_password: str) -> bool:
    return bcrypt.checkpw(password.encode(), hashed_password.encode())


def generate_access_secret() -> str:
    return "".join(secrets.choice(ALPHABET) for _ in range(ACCESS_SECRET_LENGTH))
