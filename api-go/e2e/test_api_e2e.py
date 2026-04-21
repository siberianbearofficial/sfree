from uuid import uuid4


async def test_sources_lifecycle(client, e2e_context):
    sources = await client.list_sources(e2e_context.auth)
    assert any(item["id"] == e2e_context.source_id for item in sources)

    source_info = await client.get_source_info(e2e_context.auth, e2e_context.source_id)
    assert source_info["id"] == e2e_context.source_id
    assert source_info["type"] == e2e_context.source_type


async def test_upload_download_via_s3_and_http(client, e2e_context):
    filename = f"e2e-object-{uuid4().hex[:8]}.txt"
    payload = b"sfree e2e payload"

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
    first_list = await client.list_objects_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )
    etag_before = next(item for item in first_list if item["Key"] == filename)["ETag"]

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload_v2,
    )
    second_list = await client.list_objects_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )
    etag_after = next(item for item in second_list if item["Key"] == filename)["ETag"]
    assert etag_after != etag_before

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
    payload = b"sfree e2e http payload"

    uploaded = await client.upload_file_http(e2e_context.auth, e2e_context.bucket_id, filename, payload)
    downloaded = await client.download_file_http(e2e_context.auth, e2e_context.bucket_id, uploaded["id"])
    assert downloaded == payload


async def test_s3_list_objects_v2_returns_uploaded_files(client, e2e_context):
    name_1 = f"e2e-list-{uuid4().hex[:8]}-1.txt"
    name_2 = f"e2e-list-{uuid4().hex[:8]}-2.txt"

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=name_1,
        content=b"one",
    )
    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=name_2,
        content=b"two-two",
    )

    objects = await client.list_objects_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )

    by_key = {item["Key"]: item for item in objects}
    assert name_1 in by_key
    assert name_2 in by_key
    assert by_key[name_1]["Size"] == 3
    assert by_key[name_2]["Size"] == 7


async def test_s3_sdk_list_objects_v2_prefix_delimiter_and_pagination(client, e2e_context):
    root = f"e2e-sdk-list-{uuid4().hex[:8]}"
    object_names = [
        f"{root}/docs/readme.txt",
        f"{root}/photos/2024/a.jpg",
        f"{root}/photos/2024/b.jpg",
        f"{root}/photos/2025/c.jpg",
        f"{root}/photos/root.txt",
    ]

    for object_name in object_names:
        await client.upload_file_s3(
            access_key=e2e_context.access_key,
            access_secret=e2e_context.access_secret,
            bucket_key=e2e_context.bucket_key,
            object_key=object_name,
            content=f"content {object_name}".encode(),
        )

    prefix = f"{root}/photos/"
    prefix_response = await client.list_objects_v2_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        prefix=prefix,
    )
    prefix_keys = {item["Key"] for item in prefix_response.get("Contents", [])}
    assert prefix_keys == {name for name in object_names if name.startswith(prefix)}
    assert prefix_response["KeyCount"] == 4

    delimiter_response = await client.list_objects_v2_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        prefix=prefix,
        delimiter="/",
    )
    delimiter_keys = {item["Key"] for item in delimiter_response.get("Contents", [])}
    common_prefixes = {item["Prefix"] for item in delimiter_response.get("CommonPrefixes", [])}
    assert delimiter_keys == {f"{root}/photos/root.txt"}
    assert common_prefixes == {f"{root}/photos/2024/", f"{root}/photos/2025/"}
    assert delimiter_response["KeyCount"] == 3

    first_page = await client.list_objects_v2_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        prefix=prefix,
        max_keys=2,
    )
    first_page_keys = {item["Key"] for item in first_page.get("Contents", [])}
    assert first_page["KeyCount"] == 2
    assert first_page["IsTruncated"] is True
    assert first_page["NextContinuationToken"]

    second_page = await client.list_objects_v2_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        prefix=prefix,
        max_keys=2,
        continuation_token=first_page["NextContinuationToken"],
    )
    second_page_keys = {item["Key"] for item in second_page.get("Contents", [])}
    assert second_page["KeyCount"] == 2
    assert second_page["IsTruncated"] is False
    assert first_page_keys.isdisjoint(second_page_keys)
    assert first_page_keys | second_page_keys == {name for name in object_names if name.startswith(prefix)}


async def test_s3_sdk_get_object_range_returns_partial_content(client, e2e_context):
    filename = f"e2e-sdk-range-{uuid4().hex[:8]}.txt"
    payload = b"abcdefghijklmnopqrstuvwxyz"

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload,
    )

    response = await client.get_object_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        byte_range="bytes=2-6",
    )
    assert response["ResponseMetadata"]["HTTPStatusCode"] == 206
    assert response["Body"] == b"cdefg"
    assert response["ContentLength"] == 5
    assert response["ContentRange"] == "bytes 2-6/26"
    assert response["AcceptRanges"] == "bytes"


async def test_s3_sdk_delete_objects_removes_multiple_keys(client, e2e_context):
    root = f"e2e-sdk-delete-{uuid4().hex[:8]}"
    keep_key = f"{root}/keep.txt"
    deleted_keys = [f"{root}/delete-a.txt", f"{root}/delete-b.txt"]

    for object_name in [keep_key, *deleted_keys]:
        await client.upload_file_s3(
            access_key=e2e_context.access_key,
            access_secret=e2e_context.access_secret,
            bucket_key=e2e_context.bucket_key,
            object_key=object_name,
            content=f"content {object_name}".encode(),
        )

    response = await client.delete_objects_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_keys=[*deleted_keys, f"{root}/missing.txt"],
    )
    response_deleted_keys = {item["Key"] for item in response.get("Deleted", [])}
    assert response_deleted_keys == {*deleted_keys, f"{root}/missing.txt"}

    list_response = await client.list_objects_v2_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        prefix=f"{root}/",
    )
    remaining_keys = {item["Key"] for item in list_response.get("Contents", [])}
    assert remaining_keys == {keep_key}


async def test_s3_delete_object_removes_file_metadata_and_content(client, e2e_context):
    filename = f"e2e-delete-{uuid4().hex[:8]}.txt"
    payload = b"to-be-deleted"

    await client.upload_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        content=payload,
    )

    await client.delete_object_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )

    objects = await client.list_objects_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )
    assert all(item["Key"] != filename for item in objects)

    files = await client.list_files(e2e_context.auth, e2e_context.bucket_id)
    assert all(item["name"] != filename for item in files)

    await client.delete_object_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )


async def test_delete_source_returns_conflict_while_bucket_uses_it(client, e2e_context):
    status = await client.delete_source_status(e2e_context.auth, e2e_context.source_id)
    assert status == 409


async def test_s3_sdk_multipart_upload_flow(client, e2e_context):
    filename = f"e2e-multipart-{uuid4().hex[:8]}.bin"
    part_payload = b"sfree multipart payload"

    create_resp = await client.create_multipart_upload_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )
    upload_id = create_resp["UploadId"]

    listed_uploads = await client.list_multipart_uploads_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )
    active_uploads = listed_uploads.get("Uploads", [])
    assert any(upload["UploadId"] == upload_id and upload["Key"] == filename for upload in active_uploads)

    part = await client.upload_part_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        upload_id=upload_id,
        part_number=1,
        content=part_payload,
    )
    part_etag = part["ETag"]

    listed_parts = await client.list_parts_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        upload_id=upload_id,
    )
    listed_parts = listed_parts.get("Parts", [])
    assert len(listed_parts) == 1
    assert listed_parts[0]["PartNumber"] == 1
    assert listed_parts[0]["ETag"] == part_etag

    await client.complete_multipart_upload_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
        upload_id=upload_id,
        parts=[{"PartNumber": 1, "ETag": part_etag}],
    )

    listed_uploads_after = await client.list_multipart_uploads_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
    )
    active_uploads_after = listed_uploads_after.get("Uploads", [])
    assert all(upload["UploadId"] != upload_id for upload in active_uploads_after)

    completed = await client.download_file_s3(
        access_key=e2e_context.access_key,
        access_secret=e2e_context.access_secret,
        bucket_key=e2e_context.bucket_key,
        object_key=filename,
    )
    assert completed == part_payload
