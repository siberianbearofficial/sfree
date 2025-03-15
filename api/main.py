import fastapi
import fastapi_xml

from s3.router import router as s3_router
from users.router import router as user_router
from buckets.router import router as bucket_router
from gdrive.router import router as gdrive_router
from setup.router import router as setup_router

DESCRIPTION = """
S3aaS has **S3-complatible routes** to store/retrieve/remove data and **REST API** for administrating purposes.

You can create **multiple sources** to allow S3aaS to distribute chunks of data into them under the hood.

If you don't have enough space in any of added sources but still have enough space in all of them together,
there is no problem to the system to handle file division and **use all available space in all sources**.
"""

app = fastapi.FastAPI(
    title="S3aaS",
    summary="S3-compatible API that stores data in various storage systems.",
    description=DESCRIPTION,
    contact={
        "name": "Aleksei Orlov",
        "email": "support@aleksei-orlov.ru",
        "url": "https://github.com/siberianbearofficial",
    },
    license_info={
        "name": "Apache 2.0",
        "url": "https://www.apache.org/licenses/LICENSE-2.0.html",
    },
)
app.router.route_class = fastapi_xml.XmlRoute
fastapi_xml.add_openapi_extension(app)

app.include_router(s3_router, prefix="/api/v1/s3", tags=["s3"])
app.include_router(user_router, prefix="/api/v1/users", tags=["users"])
app.include_router(bucket_router, prefix="/api/v1/buckets", tags=["buckets"])
app.include_router(gdrive_router, prefix="/api/v1/sources/gdrive", tags=["gdrive"])
app.include_router(setup_router)
