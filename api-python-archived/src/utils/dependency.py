from typing import Annotated

from fastapi import Depends

from src.buckets.service import BucketService, get_bucket_service
from src.gdrive.service import GDriveService, get_gdrive_service
from src.s3.service import S3Service, get_s3_service
from src.users.service import UserService, get_user_service
from src.utils.unitofwork import IUnitOfWork, get_uow


UserServiceDep = Annotated[UserService, Depends(get_user_service)]
BucketServiceDep = Annotated[BucketService, Depends(get_bucket_service)]
S3ServiceDep = Annotated[S3Service, Depends(get_s3_service)]
GDriveServiceDep = Annotated[GDriveService, Depends(get_gdrive_service)]

UOWDep = Annotated[IUnitOfWork, Depends(get_uow)]
