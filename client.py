import asyncio
from aiobotocore.session import get_session


AWS_ACCESS_KEY_ID = "test-bucket./}t08$?O.kcRndL.l{`<"
AWS_SECRET_ACCESS_KEY = (
    "w+]a;PT-hCvPG%^aXf_P#;89`Chvk_W$a&'}n~7Ht)Y^c)p%''^L\\pon`c4{-uOxi+B?38?`n?`\"B3d8"
)


async def main():
    session = get_session()
    endpoint_url = "http://127.0.0.1:8000/api/v1/s3"

    async with session.create_client(
        "s3",
        region_name="us-east-1",
        endpoint_url=endpoint_url,
        aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
        aws_access_key_id=AWS_ACCESS_KEY_ID,
    ) as client:
        bucket_name = "test-bucket"
        # file_name = "/Users/alekse.v.orlov/Downloads/JetBrains.Rider-2024.2.4.dmg"
        # file_name = "/Users/alekse.v.orlov/Downloads/GPT-chat-avalonia 2.dmg"
        file_name = "example.txt"

        with open(file_name, "rb") as f:
            response = await client.put_object(
                Bucket=bucket_name,
                Key="example4.txt",
                Body=f,
            )
            print("Lib Upload Response:", response)

        # response = await client.list_objects_v2(Bucket=bucket_name)
        # print(response.get("Contents"))
        # keys = [item.get("Key") for item in response.get("Contents")]
        # print(keys)

        response = await client.get_object(Bucket=bucket_name, Key="rider_2")
        content = await response["Body"].read()
        print("Downloaded Content:", content.decode())

        # response = await client.delete_object(Bucket=bucket_name, Key=file_name)
        # print("Delete Response:", response)


if __name__ == "__main__":
    asyncio.run(main())
