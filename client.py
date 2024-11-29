import asyncio
from aiobotocore.session import get_session


AWS_ACCESS_KEY_ID = "xxx"
AWS_SECRET_ACCESS_KEY = "xxx"


async def main():
    session = get_session()
    endpoint_url = "http://127.0.0.1:8000/api/v1/s3"

    async with session.create_client(
            "s3",
            region_name="us-east-1",
            endpoint_url=endpoint_url,
            aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
            aws_access_key_id=AWS_ACCESS_KEY_ID
    ) as client:
        bucket_name = "test-bucket"
        file_name = "example.txt"

        # with open(file_name, "rb") as f:
        #     response = await client.put_object(
        #         Bucket=bucket_name,
        #         Key=file_name,
        #         Body=f,
        #     )
        #     print("Lib Upload Response:", response)

        response = await client.list_objects_v2(Bucket=bucket_name)
        keys = [item.get("Key") for item in response.get("Contents")]

        response = await client.get_object(Bucket=bucket_name, Key=keys[0])
        content = await response["Body"].read()
        print("Downloaded Content:", content.decode())

        # response = await client.delete_object(Bucket=bucket_name, Key=file_name)
        # print("Delete Response:", response)


if __name__ == "__main__":
    asyncio.run(main())
