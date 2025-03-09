from utils.exceptions import NotFoundError


class GoogleDriveNoServiceError(Exception):
    def __str__(self):
        return "Google Drive service not provided. Perhaps you should use the client via async context manager."


class GoogleDriveDirectoryNotFound(NotFoundError):
    def __str__(self):
        return "Google Drive directory not found."


class GoogleDriveDirectoryCreationError(PermissionError):
    def __str__(self):
        return "Google Drive directory creation failed."


class GoogleDriveDirectoryDeletionError(PermissionError):
    def __str__(self):
        return "Google Drive directory deletion failed."


class GoogleDriveFileNotFound(NotFoundError):
    def __str__(self):
        return "Google Drive file not found."


class GoogleDriveFileUploadError(PermissionError):
    def __str__(self):
        return "Google Drive file upload failed."


class GoogleDriveFileDownloadError(PermissionError):
    def __str__(self):
        return "Google Drive file download failed."


class GoogleDriveFileUpdateError(PermissionError):
    def __str__(self):
        return "Google Drive file update failed."


class GoogleDriveFileDeletionError(PermissionError):
    def __str__(self):
        return "Google Drive file deletion failed."
