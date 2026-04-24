#!/bin/sh
set -eu

base_url="${SFREE_SMOKE_BASE_URL:-http://docker:8080}"
frontend_url="${SFREE_SMOKE_FRONTEND_URL:-http://docker:3000}"
suffix="$(date +%s)-$$"
tmpdir="$(mktemp -d)"
export COMPOSE_PROJECT_NAME="sfree_smoke_$suffix"
export AWS_EC2_METADATA_DISABLED=true
tools_image="sfree-smoke-tools:$suffix"
tools_container=""

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

pull_image() {
	image="$1"
	for i in $(seq 1 4); do
		if docker pull "$image"; then
			return 0
		fi
		sleep $((i * 10))
	done
	return 1
}

pull_image_or_fallback() {
	target_var="$1"
	primary="$2"
	fallback="$3"

	if pull_image "$primary"; then
		return 0
	fi

	if [ -n "$fallback" ] && [ "$fallback" != "$primary" ] && pull_image "$fallback"; then
		case "$target_var" in
			MINIO_IMAGE | MINIO_MC_IMAGE) ;;
			*) fail "Invalid fallback target $target_var" ;;
		esac
		eval "$target_var=\$fallback"
		export "$target_var"
		return 0
	fi

	fail "Unable to pull $primary"
}

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		compose ps || true
		compose logs --tail=160 api webui minio minio-init mongo || true
	fi
	compose down -v --remove-orphans || true
	if [ -n "$tools_container" ]; then
		docker rm -f "$tools_container" >/dev/null 2>&1 || true
	fi
	docker image rm -f "$tools_image" >/dev/null 2>&1 || true
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
docker build \
	--build-arg "GO_IMAGE=$GO_IMAGE" \
	-f api-go/Dockerfile.smoke-tools \
	-t "$tools_image" \
	api-go
tools_container="$(docker create "$tools_image")"
docker cp "$tools_container:/out/sfree" "$tmpdir/sfree"
docker cp "$tools_container:/out/smoke-helper" "$tmpdir/smoke-helper"
docker rm "$tools_container"
tools_container=""
pass "sfree CLI and smoke helper build"

step "Start Compose stack"
for image in "$GO_IMAGE" "$NODE_IMAGE" "$NGINX_IMAGE" "$MONGO_IMAGE"; do
	if ! pull_image "$image"; then
		fail "Unable to pull $image"
	fi
done
pull_image_or_fallback MINIO_IMAGE "$MINIO_IMAGE" "${MINIO_IMAGE_FALLBACK:-minio/minio:RELEASE.2025-01-20T14-49-07Z}"
pull_image_or_fallback MINIO_MC_IMAGE "$MINIO_MC_IMAGE" "${MINIO_MC_IMAGE_FALLBACK:-minio/mc:latest}"
compose up -d --pull never --build
pass "Woodpecker starts the root Compose stack"

step "Wait for API readiness"
for i in $(seq 1 120); do
	if "$tmpdir/smoke-helper" ready "$base_url/readyz"; then
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
mc_download="$tmpdir/mc-download.txt"
share_download="$tmpdir/share-download.txt"

printf 'sfree smoke payload %s\n' "$suffix" > "$payload"

step "Create user via API"
password="$("$tmpdir/smoke-helper" create-user "$base_url" "$username")"
[ -n "$password" ] || fail "User creation response did not include a password"
pass "User creation via API works"

step "Configure MinIO source via API"
source_id="$("$tmpdir/smoke-helper" create-source "$base_url" "$username" "$password" "$source_name")"
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

step "MinIO client root-endpoint download"
docker run --rm \
	--network "${COMPOSE_PROJECT_NAME}_default" \
	--entrypoint sh \
	"$MINIO_MC_IMAGE" sh -ceu '
		mc alias set --api S3v4 --path on sfree http://api:8080 "$1" "$2" >/dev/null
		mc cat "sfree/$3/$4"
	' sh "$access_key" "$access_secret" "$bucket_key" "$(basename "$payload")" > "$mc_download"
cmp "$payload" "$mc_download"
pass "Downloaded bytes match through MinIO client on the root S3 endpoint"

step "Frontend-origin public share download"
share_path="$("$tmpdir/smoke-helper" share-url "$base_url" "$username" "$password" "$bucket_id" "$file_id")"
case "$share_path" in
	/share/*) ;;
	*) fail "Share creation response did not include a /share/ URL" ;;
esac
"$tmpdir/smoke-helper" download-url "$frontend_url$share_path" "$share_download"
cmp "$payload" "$share_download"
pass "Downloaded bytes match through the frontend-origin public share URL"

pass "Woodpecker smoke validation completed"
