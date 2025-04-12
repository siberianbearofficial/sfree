import fastapi
import fastapi_xml

from src.s3.router import router as s3_router
from src.users.router import router as user_router
from src.buckets.router import router as bucket_router
from src.gdrive.router import router as gdrive_router
from src.setup.router import router as setup_router
from src.utils.exceptions import endpoints_exception_handler

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
        "name": "MIT",
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

# регистрируем функцию-обработчик ошибок
app.exception_handler(Exception)(endpoints_exception_handler)
