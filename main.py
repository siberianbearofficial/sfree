from fastapi import FastAPI
from fastapi_xml import add_openapi_extension, XmlRoute

from s3.router import router as s3_router
from admin.router import router as admin_router

app = FastAPI()
app.router.route_class = XmlRoute
add_openapi_extension(app)

app.include_router(s3_router, prefix="/api/v1/s3", tags=["s3"])
app.include_router(admin_router, prefix="/api/v1/admin", tags=["admin"])
