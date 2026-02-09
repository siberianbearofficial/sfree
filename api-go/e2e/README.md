# API Go E2E

E2E-тесты проверяют HTTP и S3-эндпоинты `api-go` через реальный клиент на `aiobotocore`.

## Что покрывается

- `POST /api/v1/users`
- `POST /api/v1/buckets`
- `GET /api/v1/buckets`
- `DELETE /api/v1/buckets/:id`
- `POST /api/v1/sources/gdrive`
- `GET /api/v1/sources`
- `GET /api/v1/sources/:id/info`
- `DELETE /api/v1/sources/:id`
- `POST /api/v1/buckets/:id/upload`
- `GET /api/v1/buckets/:id/files`
- `GET /api/v1/buckets/:id/files/:file_id/download`
- `DELETE /api/v1/buckets/:id/files/:file_id`
- `GET /api/s3/:bucket/*object`

## Локальный запуск

```bash
cd api-go
export E2E_GDRIVE_KEY='<json_service_account_or_token>'
make test-e2e-local
```

## Запуск без Docker Compose

```bash
cd api-go
export E2E_BASE_API_URL='http://localhost:8080'
export E2E_GDRIVE_KEY='<json_service_account_or_token>'
make test-e2e-python
```

## CI (Woodpecker)

Pipeline `.woodpecker/api-go.yml` запускает `docker-compose.e2e.yml` через DinD.

Нужно добавить секрет:

- `E2E_GDRIVE_KEY` — ключ для Google Drive source.

Нужно убедиться, что у агента разрешены privileged service-контейнеры (для `docker:dind`).
