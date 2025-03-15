import hashlib
import hmac
from typing import Annotated

from fastapi import Depends, Path

from buckets.schema import BucketRead
from utils.dependency import UOWDep, BucketServiceDep
from utils.exceptions import exception_handler, AuthenticationError

BucketKeyDep = Annotated[str, Path()]


@exception_handler
async def get_bucket(
    uow: UOWDep,
    bucket_service: BucketServiceDep,
    bucket_key: BucketKeyDep,
) -> BucketRead:
    bucket = await bucket_service.get_bucket(
        uow,
        bucket_key,
    )
    if not bucket:
        raise AuthenticationError("Not authenticated")

    return bucket


BucketDep = Annotated[BucketRead, Depends(get_bucket)]


def hmac_sha256(key, message):
    """Генерация HMAC-SHA256."""
    return hmac.new(key, message.encode("utf-8"), hashlib.sha256).digest()


def hash_sha256(data):
    """SHA-256 хэширование строки."""
    return hashlib.sha256(data.encode("utf-8")).hexdigest()


def parse_authorization_header(header):
    """Парсинг заголовка Authorization."""
    if not header.startswith("AWS4-HMAC-SHA256"):
        raise ValueError("Invalid Authorization header format")

    parts = header[len("AWS4-HMAC-SHA256") :].strip().split(", ")
    parsed = {}
    for part in parts:
        key, value = part.split("=")
        parsed[key] = value

    return parsed


def generate_signing_key(secret_key, date, region, service):
    """Генерация подписывающего ключа (SigningKey)."""
    date_key = hmac_sha256(f"AWS4{secret_key}".encode("utf-8"), date)
    region_key = hmac_sha256(date_key, region)
    service_key = hmac_sha256(region_key, service)
    signing_key = hmac_sha256(service_key, "aws4_request")
    return signing_key


def create_canonical_request(request, signed_headers):
    """Формирование CanonicalRequest."""
    canonical_headers = "".join(
        f"{header}:{request.headers[header].strip()}\n" for header in signed_headers
    )
    payload_hash = request.headers.get("x-amz-content-sha256", hash_sha256(""))

    canonical_request = f"""{request.method}
{request.url.path}
{request.url.query}
{canonical_headers}
{";".join(signed_headers)}
{payload_hash}"""
    return canonical_request


def create_string_to_sign(canonical_request, timestamp, credential_scope):
    """Формирование StringToSign."""
    canonical_request_hash = hash_sha256(canonical_request)
    string_to_sign = f"""AWS4-HMAC-SHA256
{timestamp}
{credential_scope}
{canonical_request_hash}"""
    return string_to_sign


def verify_signature(request, secret_key):
    """Проверка подписи запроса с отладочной информацией."""
    # 1. Парсим заголовок Authorization
    auth_header = request.headers.get("Authorization")
    if not auth_header or not auth_header.startswith("AWS4-HMAC-SHA256"):
        raise ValueError("Invalid or missing Authorization header")

    auth_parts = parse_authorization_header(auth_header)
    print(auth_header)
    credential_parts = auth_parts["Credential"].split("/")
    access_key = credential_parts[0]
    date = credential_parts[1]
    region = credential_parts[2]
    service = credential_parts[3]
    signed_headers = auth_parts["SignedHeaders"].split(";")
    client_signature = auth_parts["Signature"]

    # 2. Проверяем заголовки
    for header in signed_headers:
        if header not in request.headers:
            raise ValueError(f"Missing signed header: {header}")

    # 3. Генерируем SigningKey
    signing_key = generate_signing_key(secret_key, date, region, service)

    # 4. Формируем CanonicalRequest
    canonical_request = create_canonical_request(request, signed_headers)

    # 5. Формируем StringToSign
    credential_scope = f"{access_key}/{date}/{region}/{service}/aws4_request"
    timestamp = request.headers["x-amz-date"]
    string_to_sign = create_string_to_sign(canonical_request, timestamp, credential_scope)

    # 6. Вычисляем подпись
    server_signature = hmac_sha256(signing_key, string_to_sign).hex()

    # 7. Логируем отладочную информацию
    print("=== DEBUG ===")
    print(f"CanonicalRequest:\n{canonical_request}")
    print(f"StringToSign:\n{string_to_sign}")
    print(f"ServerSignature: {server_signature}")
    print(f"ClientSignature: {client_signature}")
    print("=================")

    # 8. Сравниваем подписи
    if server_signature != client_signature:
        raise ValueError("Signature does not match")
