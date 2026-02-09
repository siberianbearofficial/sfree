from uuid import uuid4

from aiohttp import BasicAuth


async def test_http_and_s3_endpoints(client):
    username = f"e2e-user-{uuid4().hex[:10]}"
    bucket_key = f"e2e-bucket-{uuid4().hex[:10]}"
    source_name = f"e2e-source-{uuid4().hex[:10]}"
    filename = f"e2e-object-{uuid4().hex[:8]}.txt"
    payload = b"s3aas e2e payload"

    user = await client.create_user(username)
    assert user["id"]
    assert user["password"]
    auth = BasicAuth(login=username, password=user["password"])

    bucket = await client.create_bucket(auth=auth, key=bucket_key)
    assert bucket["key"] == bucket_key
    assert bucket["access_key"]
    assert bucket["access_secret"]

    buckets = await client.list_buckets(auth)
    bucket_from_list = next(item for item in buckets if item["key"] == bucket_key)
    bucket_id = bucket_from_list["id"]

    source = await client.create_gdrive_source(auth, source_name)
    assert source["name"] == source_name
    source_id = source["id"]

    sources = await client.list_sources(auth)
    assert any(item["id"] == source_id for item in sources)

    source_info = await client.get_source_info(auth, source_id)
    assert source_info["id"] == source_id
    assert source_info["type"] == "gdrive"

    uploaded = await client.upload_file_http(auth, bucket_id, filename, payload)
    file_id = uploaded["id"]

    files = await client.list_files(auth, bucket_id)
    assert any(item["id"] == file_id and item["name"] == filename for item in files)

    downloaded_http = await client.download_file_http(auth, bucket_id, file_id)
    assert downloaded_http == payload

    downloaded_s3 = await client.download_file_s3(
        access_key=bucket["access_key"],
        access_secret=bucket["access_secret"],
        bucket_key=bucket_key,
        object_key=filename,
    )
    assert downloaded_s3 == payload

    await client.delete_file(auth, bucket_id, file_id)
    assert not await client.list_files(auth, bucket_id)

    await client.delete_source(auth, source_id)
    assert not await client.list_sources(auth)

    await client.delete_bucket(auth, bucket_id)
    assert not await client.list_buckets(auth)
