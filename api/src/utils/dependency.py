from typing import Annotated

from fastapi import Depends

from buckets.service import BucketService, get_bucket_service
from gdrive.service import GDriveService, get_gdrive_service
from s3.service import S3Service, get_s3_service
from users.service import UserService, get_user_service
from utils.unitofwork import IUnitOfWork, get_uow


UserServiceDep = Annotated[UserService, Depends(get_user_service)]
BucketServiceDep = Annotated[BucketService, Depends(get_bucket_service)]
S3ServiceDep = Annotated[S3Service, Depends(get_s3_service)]
GDriveServiceDep = Annotated[GDriveService, Depends(get_gdrive_service)]

UOWDep = Annotated[IUnitOfWork, Depends(get_uow)]
