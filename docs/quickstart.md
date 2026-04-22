# Docker Compose Quickstart

Get SFree running locally in under 5 minutes. By the end you will have
uploaded a file, downloaded it back, and optionally accessed it through an
S3-compatible endpoint.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (v20+)
- [Docker Compose](https://docs.docker.com/compose/install/) (v2+, included
  with Docker Desktop)

> **Windows / macOS:** Install Docker Desktop — it bundles Docker Compose.
>
> **Linux:** Install Docker Engine and the Compose plugin
> (`docker compose` — no hyphen).

## 1. Clone and start

```bash
git clone https://github.com/siberianbearofficial/sfree.git
cd sfree
docker compose up --build
```

Wait until you see log lines from all services (`mongo`, `minio`, `api`,
`webui`). First build takes 2–3 minutes; subsequent starts are faster.

**Expected output (last few lines):**

```
webui-1  | /docker-entrypoint.sh: Configuration complete; ready for start up
api-1    | [GIN-debug] Listening and serving HTTP on :8080
```

| Service       | URL                                       |
| ------------- | ----------------------------------------- |
| Frontend      | http://localhost:3000                      |
| REST API      | http://localhost:8080                      |
| API docs      | http://localhost:8080/api/docs             |
| MinIO Console | http://localhost:9001                      |

## 2. Create a user

```bash
curl -s -X POST http://localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"username": "demo"}'
```

**Expected output:**

```json
{
  "id": "6...",
  "created_at": "2025-...",
  "password": "GENERATED_PASSWORD"
}
```

Save the `password` — you need it for every authenticated request.

Now encode your credentials for Basic Auth:

```bash
# Replace GENERATED_PASSWORD with the password from the response above
AUTH=$(echo -n 'demo:GENERATED_PASSWORD' | base64)
echo $AUTH
```

You will use `$AUTH` in the commands below.

> **Tip:** You can also sign up through the browser UI at
> http://localhost:3000 — click **Get started** and create an account. The
> UI displays your password once on signup.

## 3. Add a storage source

The Docker Compose stack includes a MinIO instance pre-configured with an
`sfree-data` bucket. Register it as an S3-compatible source:

```bash
curl -s -X POST http://localhost:8080/api/v1/sources/s3 \
  -H 'Content-Type: application/json' \
  -H "Authorization: Basic $AUTH" \
  -d '{
    "name": "local-minio",
    "endpoint": "http://minio:9000",
    "bucket": "sfree-data",
    "access_key_id": "minioadmin",
    "secret_access_key": "minioadmin",
    "region": "us-east-1",
    "path_style": true
  }'
```

**Expected output:**

```json
{
  "id": "SOURCE_ID",
  "name": "local-minio",
  ...
}
```

Save the `id` — you need it to create a bucket.

> **Other source types:** SFree also supports Google Drive and Telegram
> sources. See the [API docs](http://localhost:8080/api/docs) for
> details.

## 4. Create a bucket

A bucket groups one or more sources into a single storage namespace.

```bash
curl -s -X POST http://localhost:8080/api/v1/buckets \
  -H 'Content-Type: application/json' \
  -H "Authorization: Basic $AUTH" \
  -d '{
    "key": "my-bucket",
    "source_ids": ["SOURCE_ID"]
  }'
```

Replace `SOURCE_ID` with the source id from step 3.

**Expected output:**

```json
{
  "key": "my-bucket",
  "access_key": "...",
  "access_secret": "...",
  "created_at": "..."
}
```

Save `access_key` and `access_secret` — these are S3 credentials for this
bucket (used in step 7).

Now fetch the bucket id used by the REST upload and download endpoints:

```bash
curl -s http://localhost:8080/api/v1/buckets \
  -H "Authorization: Basic $AUTH"
```

**Expected output:**

```json
[
  {
    "id": "BUCKET_ID",
    "key": "my-bucket",
    "access_key": "my-bucket",
    "created_at": "...",
    "role": "owner",
    "shared": false
  }
]
```

Set `BUCKET_ID` to the `id` for `my-bucket`:

```bash
BUCKET_ID=BUCKET_ID
```

## 5. Upload a file

```bash
curl -s -X POST "http://localhost:8080/api/v1/buckets/$BUCKET_ID/upload" \
  -H "Authorization: Basic $AUTH" \
  -F 'file=@README.md'
```

**Expected output:**

```json
{
  "id": "FILE_ID",
  "name": "README.md",
  "created_at": "..."
}
```

## 6. Download the file

```bash
curl -s "http://localhost:8080/api/v1/buckets/$BUCKET_ID/files/FILE_ID/download" \
  -H "Authorization: Basic $AUTH" \
  -o downloaded-README.md
```

Verify the contents match:

```bash
diff README.md downloaded-README.md
```

No output means the files are identical — your upload and download worked.

## 7. (Optional) Access via S3-compatible endpoint

Every SFree bucket exposes an S3-compatible interface. Use the `access_key`
and `access_secret` from step 4 with any S3-compatible client.

### With aws-cli

```bash
# Configure a named profile
aws configure set aws_access_key_id ACCESS_KEY --profile sfree
aws configure set aws_secret_access_key ACCESS_SECRET --profile sfree
aws configure set region us-east-1 --profile sfree

# List objects
aws s3 ls s3://my-bucket/ \
  --endpoint-url http://localhost:8080/api/s3 \
  --profile sfree

# Download an object
aws s3 cp s3://my-bucket/README.md ./s3-download.md \
  --endpoint-url http://localhost:8080/api/s3 \
  --profile sfree
```

Replace `ACCESS_KEY` and `ACCESS_SECRET` with the values from step 4.

## Clean up

Stop and remove all containers and volumes:

```bash
docker compose down -v
```

## What's next

- **Add more sources** — connect Google Drive, Telegram, or another
  S3-compatible service to distribute files across multiple backends.
- **Explore the UI** — manage buckets and files at http://localhost:3000.
- **Read the architecture** — see [docs/architecture.md](architecture.md)
  for how chunking and round-robin distribution work.
- **Contribute** — check [CONTRIBUTING.md](../CONTRIBUTING.md) for
  guidelines.
