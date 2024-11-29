import hashlib
import hmac
import mimetypes
from typing import Optional

from fastapi import APIRouter, Request, HTTPException
from fastapi.responses import FileResponse
from fastapi_xml import XmlAppResponse

from schemas import ListBucketResultContents, ListBucketResult, PutObjectResult, DeleteResultDeleted, DeleteResult
from utils.config import BASE_DIR, SECRET_KEY

router = APIRouter()


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

    parts = header[len("AWS4-HMAC-SHA256"):].strip().split(", ")
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


@router.get("/{bucket_name}")
async def list_objects(
        request: Request,
        bucket_name: str,
        continuation_token: Optional[str] = None,
        delimiter: Optional[str] = None,
        encoding_type: str = "url",
        max_keys: int = 1000,
        prefix: str = "",
        start_after: Optional[str] = None,
        list_type: int = 2,
):
    """ListObjectsV2 S3 совместимый запрос."""

    try:
        verify_signature(request, secret_key=SECRET_KEY)
    except ValueError as e:
        print(e)
        raise HTTPException(status_code=401, detail=str(e))

    bucket_path = BASE_DIR / bucket_name

    if not bucket_path.exists():
        raise HTTPException(status_code=404, detail="Bucket does not exist.")

    all_objects = []
    for item in bucket_path.glob("**/*"):
        if item.is_file():
            relative_path = str(item.relative_to(bucket_path))
            if relative_path.startswith(prefix):
                all_objects.append(relative_path)

    start_index = 0
    if continuation_token:
        try:
            start_index = all_objects.index(continuation_token) + 1
        except ValueError:
            raise HTTPException(status_code=400, detail="Continuation Token is invalid.")

    objects = all_objects[start_index: start_index + max_keys]
    is_truncated = len(all_objects) > start_index + max_keys

    return XmlAppResponse(
        ListBucketResult(
            Name=bucket_name,
            Prefix=prefix,
            KeyCount=len(objects),
            MaxKeys=max_keys,
            IsTruncated=is_truncated,
            Delimiter=delimiter,
            EncodingType=encoding_type,
            ContinuationToken=continuation_token,
            NextContinuationToken=objects[-1],
            StartAfter=start_after,
            Contents=[
                ListBucketResultContents(
                    Key=obj,
                    Size=(bucket_path / obj).stat().st_size,
                    ETag=hashlib.md5((bucket_path / obj).read_bytes()).hexdigest(),
                    LastModified="1970-01-01T00:00:00Z",
                    StorageClass="STANDARD",
                ) for obj in objects
            ],
            CommonPrefixes=None,
        )
    )


@router.get("/{bucket_name}/{object_name:path}")
async def get_object(bucket_name: str, object_name: str):
    """GET Object S3 совместимый запрос."""
    bucket_path = BASE_DIR / bucket_name
    file_path = bucket_path / object_name

    if not file_path.exists():
        raise HTTPException(status_code=404, detail="The specified key does not exist")

    etag = hashlib.md5(file_path.read_bytes()).hexdigest()

    content_type, _ = mimetypes.guess_type(file_path)

    headers = {
        "Content-Type": content_type or "application/octet-stream",
        "ETag": etag,
        "Content-Length": str(file_path.stat().st_size),
    }
    return FileResponse(file_path, headers=headers)


@router.put("/{bucket_name}/{object_name:path}")
async def put_object(
        bucket_name: str,
        object_name: str,
        request: Request
):
    """PUT Object S3 совместимый запрос."""

    body = await request.body()

    bucket_path = BASE_DIR / bucket_name
    bucket_path.mkdir(parents=True, exist_ok=True)
    file_path = bucket_path / object_name

    try:
        with open(file_path, "wb") as f:
            f.write(body)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error saving object: {e}")

    etag = hashlib.md5(body).hexdigest()

    return XmlAppResponse(
        PutObjectResult(ETag=etag)
    )


@router.delete("/{bucket_name}/{object_name:path}")
async def delete_object(
        bucket_name: str,
        object_name: str,
):
    """DELETE Object S3 совместимый запрос."""

    bucket_path = BASE_DIR / bucket_name
    file_path = bucket_path / object_name

    if not file_path.exists():
        raise HTTPException(status_code=404, detail="Object does not exist")

    try:
        file_path.unlink()
    except Exception as e:
        raise e

    return XmlAppResponse(
        DeleteResult(Deleted=DeleteResultDeleted(Key=object_name))
    )
