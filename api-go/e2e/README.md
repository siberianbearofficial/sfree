# API Go E2E

E2E-тесты проверяют HTTP и S3-эндпоинты `api-go` через реальный клиент на `aiobotocore`.

## Что покрывается

- `POST /api/v1/users`
- `POST /api/v1/buckets`
- `GET /api/v1/buckets`
- `DELETE /api/v1/buckets/:id`
- `POST /api/v1/sources/gdrive`
- `POST /api/v1/sources/telegram`
- `POST /api/v1/sources/s3`
- `GET /api/v1/sources`
- `GET /api/v1/sources/:id/info`
- `DELETE /api/v1/sources/:id`
- `POST /api/v1/buckets/:id/upload`
- `GET /api/v1/buckets/:id/files`
- `GET /api/v1/buckets/:id/files/:file_id/download`
- `DELETE /api/v1/buckets/:id/files/:file_id`
- `PUT /api/s3/:bucket/*object`
- `GET /api/s3/:bucket/*object`
- `GET /api/s3/:bucket/*object` with `Range`
- `DELETE /api/s3/:bucket/*object`
- `GET /api/s3/:bucket` with ListObjectsV2 prefix, delimiter, and pagination
- `POST /api/s3/:bucket?delete`
- `POST /api/s3/:bucket/*object?uploads`
- `PUT /api/s3/:bucket/*object?partNumber&uploadId`
- `GET /api/s3/:bucket?uploads`
- `GET /api/s3/:bucket/*object?uploadId`
- `POST /api/s3/:bucket/*object?uploadId`

## Локальный запуск

```bash
cd api-go
export E2E_SOURCE_TYPE='telegram'
export E2E_TELEGRAM_TOKEN='<bot_token>'
export E2E_TELEGRAM_CHAT_ID='<chat_id>'
make test-e2e-local
```

Для Google Drive:

```bash
cd api-go
export E2E_SOURCE_TYPE='gdrive'
export E2E_GDRIVE_KEY='<json_service_account_or_token>'
make test-e2e-local
```


Для S3-compatible (через локальный MinIO из `docker-compose.e2e.yml`):

```bash
cd api-go
export E2E_SOURCE_TYPE='s3'
make test-e2e-local
```

## Запуск без Docker Compose

```bash
cd api-go
export E2E_BASE_API_URL='http://localhost:8080'
export E2E_SOURCE_TYPE='telegram'
export E2E_TELEGRAM_TOKEN='<bot_token>'
export E2E_TELEGRAM_CHAT_ID='<chat_id>'
make test-e2e-python
```

## CI (Woodpecker)

Pipeline `.woodpecker/api-go.yml` запускает `docker-compose.e2e.yml` через DinD только для `s3`. Этот режим использует локальный MinIO из `docker-compose.e2e.yml` и является обязательным PR-gate.

Режимы `gdrive` и `telegram` остаются доступными для ручной или неблокирующей проверки. Для них нужны секреты:

- `E2E_GDRIVE_KEY` — ключ для Google Drive source.
- `E2E_TELEGRAM_TOKEN` — токен Telegram-бота.
- `E2E_TELEGRAM_CHAT_ID` — id чата для отправки чанков.

Для обязательного `s3` CI-run отдельные секреты не требуются.
При необходимости можно переопределить `E2E_S3_*` переменные окружения и использовать другой S3-compatible endpoint.
