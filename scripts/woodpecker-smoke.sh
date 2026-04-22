#!/bin/sh
set -eu

base_url="${SFREE_SMOKE_BASE_URL:-http://docker:8080}"
frontend_url="${SFREE_SMOKE_FRONTEND_URL:-http://docker:3000}"
suffix="$(date +%s)-$$"
tmpdir="$(mktemp -d)"
export COMPOSE_PROJECT_NAME="sfree_smoke_$suffix"
export AWS_CONFIG_FILE="$tmpdir/aws-config"
export AWS_EC2_METADATA_DISABLED=true

compose() {
	docker compose -f docker-compose.yml "$@"
}

step() {
	printf '\nSTEP: %s\n' "$1"
}

pass() {
	printf 'PASS: %s\n' "$1"
}

fail() {
	printf 'FAIL: %s\n' "$1" >&2
	exit 1
}

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		compose ps || true
		compose logs --tail=160 api webui minio minio-init mongo || true
	fi
	compose down -v --remove-orphans || true
	rm -rf "$tmpdir"
	exit "$status"
}

trap cleanup EXIT INT TERM

step "Docker daemon availability"
for i in $(seq 1 60); do
	if docker info >/dev/null 2>&1; then
		pass "Docker daemon is reachable from Woodpecker"
		break
	fi
	if [ "$i" -eq 60 ]; then
		fail "Docker daemon did not become available"
	fi
	sleep 1
done

step "Build CLI"
(cd api-go && go build -o "$tmpdir/sfree" ./cmd/sfree-cli)
pass "sfree CLI builds"

step "Start Compose stack"
for image in "$GO_IMAGE" "$NODE_IMAGE" "$NGINX_IMAGE" "$MONGO_IMAGE" "$MINIO_IMAGE" "$MINIO_MC_IMAGE"; do
	for i in $(seq 1 4); do
		if docker pull "$image"; then
			break
		fi
		if [ "$i" -eq 4 ]; then
			fail "Unable to pull $image"
		fi
		sleep $((i * 10))
	done
done
compose up -d --pull never --build
pass "Woodpecker starts the root Compose stack"

step "Wait for API readiness"
for i in $(seq 1 120); do
	if curl -fsS "$base_url/readyz" >/dev/null; then
		pass "API is ready"
		break
	fi
	if [ "$i" -eq 120 ]; then
		fail "API did not become ready"
	fi
	sleep 1
done

username="smoke-user-$suffix"
source_name="smoke-source-$suffix"
bucket_key="smoke-bucket-$suffix"
payload="$tmpdir/payload.txt"
cli_download="$tmpdir/cli-download.txt"
s3_download="$tmpdir/s3-download.txt"
share_download="$tmpdir/share-download.txt"

printf 'sfree smoke payload %s\n' "$suffix" > "$payload"

step "Create user via API"
user_payload="$(jq -n --arg username "$username" '{username: $username}')"
user_json="$(curl -fsS -H 'Content-Type: application/json' --data "$user_payload" "$base_url/api/v1/users")"
password="$(printf '%s' "$user_json" | jq -r '.password // empty')"
[ -n "$password" ] || fail "User creation response did not include a password"
pass "User creation via API works"

step "Configure MinIO source via API"
source_payload="$(jq -n \
	--arg name "$source_name" \
	--arg endpoint "http://minio:9000" \
	--arg bucket "sfree-data" \
	--arg access_key_id "minioadmin" \
	--arg secret_access_key "minioadmin" \
	'{name: $name, endpoint: $endpoint, bucket: $bucket, access_key_id: $access_key_id, secret_access_key: $secret_access_key, region: "us-east-1", path_style: true}')"
source_json="$(curl -fsS -u "$username:$password" -H 'Content-Type: application/json' --data "$source_payload" "$base_url/api/v1/sources/s3")"
source_id="$(printf '%s' "$source_json" | jq -r '.id // empty')"
[ -n "$source_id" ] || fail "S3 source creation response did not include an id"
pass "S3-compatible MinIO source can be configured"

export SFREE_SERVER="$base_url"
export SFREE_USER="$username"
export SFREE_PASSWORD="$password"

step "CLI sources list"
sources_output="$("$tmpdir/sfree" sources list)"
printf '%s\n' "$sources_output" | grep "$source_id" >/dev/null || fail "Created source was not listed by CLI"
pass "sfree sources list works"

step "Create bucket with CLI"
bucket_output="$("$tmpdir/sfree" buckets create --key "$bucket_key" --sources "$source_id")"
access_key="$(printf '%s\n' "$bucket_output" | awk -F': *' '/Access Key:/ {print $2; exit}')"
access_secret="$(printf '%s\n' "$bucket_output" | awk -F': *' '/Access Secret:/ {print $2; exit}')"
[ -n "$access_key" ] || fail "Bucket creation output did not include an access key"
[ -n "$access_secret" ] || fail "Bucket creation output did not include an access secret"
pass "A bucket can be created"

step "CLI buckets list"
buckets_output="$("$tmpdir/sfree" buckets list)"
bucket_id="$(printf '%s\n' "$buckets_output" | awk -v key="$bucket_key" '$2 == key {print $1; exit}')"
[ -n "$bucket_id" ] || fail "Created bucket was not listed by CLI"
pass "sfree buckets list works"

step "CLI upload"
upload_output="$("$tmpdir/sfree" upload "$bucket_id" "$payload")"
file_id="$(printf '%s\n' "$upload_output" | awk -F': *' '/File ID:/ {print $2; exit}')"
[ -n "$file_id" ] || fail "CLI upload output did not include a file id"
pass "sfree upload works"

step "CLI download"
"$tmpdir/sfree" download "$bucket_id" "$file_id" "$cli_download"
cmp "$payload" "$cli_download"
pass "sfree download returns matching bytes"

step "S3 credential download"
mkdir -p "$(dirname "$AWS_CONFIG_FILE")"
printf '[default]\ns3 =\n    addressing_style = path\n' > "$AWS_CONFIG_FILE"
AWS_ACCESS_KEY_ID="$access_key" \
AWS_SECRET_ACCESS_KEY="$access_secret" \
AWS_DEFAULT_REGION=us-east-1 \
	aws --endpoint-url "$base_url/api/s3" s3api get-object \
	--bucket "$bucket_key" \
	--key "$(basename "$payload")" \
	"$s3_download" >/dev/null
cmp "$payload" "$s3_download"
pass "Downloaded bytes match through S3-compatible credentials"

step "Frontend-origin public share download"
share_json="$(curl -fsS -u "$username:$password" -H 'Content-Type: application/json' --data '{}' "$base_url/api/v1/buckets/$bucket_id/files/$file_id/share")"
share_path="$(printf '%s' "$share_json" | jq -r '.url // empty')"
case "$share_path" in
	/share/*) ;;
	*) fail "Share creation response did not include a /share/ URL" ;;
esac
curl -fsS "$frontend_url$share_path" -o "$share_download"
cmp "$payload" "$share_download"
pass "Downloaded bytes match through the frontend-origin public share URL"

pass "Woodpecker smoke validation completed"
