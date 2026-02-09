from uuid import uuid4


async def test_sources_lifecycle(client, e2e_context):
    sources = await client.list_sources(e2e_context.auth)
    assert any(item["id"] == e2e_context.source_id for item in sources)

    source_info = await client.get_source_info(e2e_context.auth, e2e_context.source_id)
    assert source_info["id"] == e2e_context.source_id
    assert source_info["type"] == "gdrive"


async def test_upload_download_via_s3_and_http(client, e2e_context):
    filename = f"e2e-object-{uuid4().hex[:8]}.txt"
    payload = b"s3aas e2e payload"

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload,
    )

    files = await client.list_files(e2e_context.auth, e2e_context.bucket_id)
    file_doc = next(item for item in files if item["name"] == filename)

    downloaded_http = await client.download_file_http(e2e_context.auth, e2e_context.bucket_id, file_doc["id"])
    assert downloaded_http == payload

    downloaded_s3 = await client.download_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )
    assert downloaded_s3 == payload


async def test_s3_put_overwrites_existing_object(client, e2e_context):
    filename = f"e2e-overwrite-{uuid4().hex[:8]}.txt"
    payload_v1 = b"payload-version-1"
    payload_v2 = b"payload-version-2"

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload_v1,
    )
    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload_v2,
    )

    files = await client.list_files(e2e_context.auth, e2e_context.bucket_id)
    matched = [item for item in files if item["name"] == filename]
    assert len(matched) == 1

    file_id = matched[0]["id"]
    downloaded_http = await client.download_file_http(e2e_context.auth, e2e_context.bucket_id, file_id)
    assert downloaded_http == payload_v2

    downloaded_s3 = await client.download_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )
    assert downloaded_s3 == payload_v2


async def test_http_upload_works(client, e2e_context):
    filename = f"e2e-http-{uuid4().hex[:8]}.txt"
    payload = b"s3aas e2e http payload"

    uploaded = await client.upload_file_http(e2e_context.auth, e2e_context.bucket_id, filename, payload)
    downloaded = await client.download_file_http(e2e_context.auth, e2e_context.bucket_id, uploaded["id"])
    assert downloaded == payload
