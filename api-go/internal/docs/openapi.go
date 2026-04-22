package docs

const openAPIJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "SFree API",
    "version": "1.0.0",
    "description": "REST and S3-compatible API surface for SFree. The S3-compatible endpoints document the subset currently implemented by api-go."
  },
  "servers": [
    {
      "url": "/"
    }
  ],
  "tags": [
    {
      "name": "health"
    },
    {
      "name": "docs"
    },
    {
      "name": "auth"
    },
    {
      "name": "users"
    },
    {
      "name": "buckets"
    },
    {
      "name": "bucket-grants"
    },
    {
      "name": "files"
    },
    {
      "name": "shares"
    },
    {
      "name": "sources"
    },
    {
      "name": "s3"
    }
  ],
  "paths": {
    "/api/openapi.json": {
      "get": {
        "tags": [
          "docs"
        ],
        "summary": "Get the OpenAPI 3 specification",
        "responses": {
          "200": {
            "description": "OpenAPI document",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object"
                }
              }
            }
          }
        }
      }
    },
    "/readyz": {
      "get": {
        "tags": [
          "health"
        ],
        "summary": "Readiness probe",
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          }
        }
      }
    },
    "/healthz": {
      "get": {
        "tags": [
          "health"
        ],
        "summary": "Health probe",
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          }
        }
      }
    },
    "/publication/ready": {
      "get": {
        "tags": [
          "health"
        ],
        "summary": "Publication readiness probe",
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          }
        }
      }
    },
    "/dbz": {
      "get": {
        "tags": [
          "health"
        ],
        "summary": "Database connectivity probe",
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "503": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/users": {
      "post": {
        "tags": [
          "users"
        ],
        "summary": "Create user",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateUserRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "User created",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/CreateUserResponse"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "409": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/auth/github": {
      "get": {
        "tags": [
          "auth"
        ],
        "summary": "Start GitHub OAuth login",
        "responses": {
          "307": {
            "description": "Redirect to GitHub OAuth"
          },
          "501": {
            "$ref": "#/components/responses/JSONError"
          }
        }
      }
    },
    "/api/v1/auth/github/callback": {
      "get": {
        "tags": [
          "auth"
        ],
        "summary": "Complete GitHub OAuth login",
        "parameters": [
          {
            "name": "code",
            "in": "query",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "state",
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "307": {
            "description": "Redirect to frontend callback"
          },
          "400": {
            "$ref": "#/components/responses/JSONError"
          }
        }
      }
    },
    "/api/v1/auth/me": {
      "get": {
        "tags": [
          "auth"
        ],
        "summary": "Get current user",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "responses": {
          "200": {
            "description": "Current user",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/CurrentUser"
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/auth/token": {
      "post": {
        "tags": [
          "auth"
        ],
        "summary": "Issue a JWT for an authenticated session",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "responses": {
          "200": {
            "description": "JWT issued",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/TokenResponse"
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets": {
      "get": {
        "tags": [
          "buckets"
        ],
        "summary": "List buckets",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "responses": {
          "200": {
            "description": "Buckets",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Bucket"
                  }
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      },
      "post": {
        "tags": [
          "buckets"
        ],
        "summary": "Create bucket",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateBucketRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Bucket created",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/CreateBucketResponse"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "409": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}": {
      "delete": {
        "tags": [
          "buckets"
        ],
        "summary": "Delete bucket",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/distribution": {
      "patch": {
        "tags": [
          "buckets"
        ],
        "summary": "Update bucket distribution",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateBucketDistributionRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "400": {
            "$ref": "#/components/responses/JSONError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/grants": {
      "get": {
        "tags": [
          "bucket-grants"
        ],
        "summary": "List bucket grants",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "description": "Bucket grants",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Grant"
                  }
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      },
      "post": {
        "tags": [
          "bucket-grants"
        ],
        "summary": "Grant bucket access",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateGrantRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Grant",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Grant"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/JSONError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "409": {
            "$ref": "#/components/responses/JSONError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/grants/{grant_id}": {
      "patch": {
        "tags": [
          "bucket-grants"
        ],
        "summary": "Update grant role",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/GrantID"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/UpdateGrantRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "400": {
            "$ref": "#/components/responses/JSONError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      },
      "delete": {
        "tags": [
          "bucket-grants"
        ],
        "summary": "Revoke grant",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/GrantID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/upload": {
      "post": {
        "tags": [
          "files"
        ],
        "summary": "Upload file to bucket",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "$ref": "#/components/schemas/FileUploadRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Uploaded file",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/File"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/files": {
      "get": {
        "tags": [
          "files"
        ],
        "summary": "List files in bucket",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "description": "Files",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/File"
                  }
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/files/{file_id}/download": {
      "get": {
        "tags": [
          "files"
        ],
        "summary": "Download bucket file",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/FileID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/BinaryFile"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/files/{file_id}": {
      "delete": {
        "tags": [
          "files"
        ],
        "summary": "Delete bucket file",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/FileID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/files/{file_id}/share": {
      "post": {
        "tags": [
          "shares"
        ],
        "summary": "Create share link",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/FileID"
          }
        ],
        "requestBody": {
          "required": false,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateShareRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Share link",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ShareLink"
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/buckets/{id}/files/{file_id}/shares": {
      "get": {
        "tags": [
          "shares"
        ],
        "summary": "List share links for a file",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/FileID"
          }
        ],
        "responses": {
          "200": {
            "description": "Share links",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/ShareLink"
                  }
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/shares/{id}": {
      "delete": {
        "tags": [
          "shares"
        ],
        "summary": "Delete share link",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/share/{token}": {
      "get": {
        "tags": [
          "shares"
        ],
        "summary": "Download shared file",
        "parameters": [
          {
            "name": "token",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/BinaryFile"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          },
          "410": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources": {
      "get": {
        "tags": [
          "sources"
        ],
        "summary": "List storage sources",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "responses": {
          "200": {
            "description": "Sources",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Source"
                  }
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/gdrive": {
      "post": {
        "tags": [
          "sources"
        ],
        "summary": "Create Google Drive source",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateGDriveSourceRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Source",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Source"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/telegram": {
      "post": {
        "tags": [
          "sources"
        ],
        "summary": "Create Telegram source",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateTelegramSourceRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Source",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Source"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/s3": {
      "post": {
        "tags": [
          "sources"
        ],
        "summary": "Create S3-compatible source",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateS3SourceRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Source",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Source"
                }
              }
            }
          },
          "400": {
            "$ref": "#/components/responses/PlainError"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/{id}": {
      "delete": {
        "tags": [
          "sources"
        ],
        "summary": "Delete storage source",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/PlainOK"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/{id}/info": {
      "get": {
        "tags": [
          "sources"
        ],
        "summary": "Get storage source file and capacity info",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          }
        ],
        "responses": {
          "200": {
            "description": "Source info",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/SourceInfo"
                }
              }
            }
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/v1/sources/{id}/files/{file_id}/download": {
      "get": {
        "tags": [
          "sources"
        ],
        "summary": "Download source file",
        "security": [
          {
            "basicAuth": []
          },
          {
            "bearerAuth": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/ID"
          },
          {
            "$ref": "#/components/parameters/FileID"
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/BinaryFile"
          },
          "401": {
            "$ref": "#/components/responses/PlainError"
          },
          "404": {
            "$ref": "#/components/responses/PlainError"
          }
        }
      }
    },
    "/api/s3/{bucket}": {
      "get": {
        "tags": [
          "s3"
        ],
        "summary": "List objects or multipart uploads",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ListType"
          },
          {
            "$ref": "#/components/parameters/Prefix"
          },
          {
            "$ref": "#/components/parameters/Delimiter"
          },
          {
            "$ref": "#/components/parameters/MaxKeys"
          },
          {
            "$ref": "#/components/parameters/ContinuationToken"
          },
          {
            "$ref": "#/components/parameters/StartAfter"
          },
          {
            "name": "uploads",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to list multipart uploads."
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/S3XML"
          },
          "400": {
            "$ref": "#/components/responses/S3Error"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      },
      "post": {
        "tags": [
          "s3"
        ],
        "summary": "Delete multiple objects",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "name": "delete",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to invoke DeleteObjects."
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/xml": {
              "schema": {
                "type": "string"
              }
            }
          }
        },
        "responses": {
          "200": {
            "$ref": "#/components/responses/S3XML"
          },
          "400": {
            "$ref": "#/components/responses/S3Error"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      }
    },
    "/api/s3/{bucket}/{objectKey}": {
      "head": {
        "tags": [
          "s3"
        ],
        "summary": "Read object metadata",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ObjectKey"
          }
        ],
        "responses": {
          "200": {
            "description": "Object headers"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      },
      "get": {
        "tags": [
          "s3"
        ],
        "summary": "Get object or list multipart upload parts",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ObjectKey"
          },
          {
            "$ref": "#/components/parameters/Range"
          },
          {
            "name": "uploadId",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to list uploaded parts instead of reading object content."
          }
        ],
        "responses": {
          "200": {
            "$ref": "#/components/responses/BinaryOrS3XML"
          },
          "206": {
            "$ref": "#/components/responses/BinaryFile"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          },
          "416": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      },
      "put": {
        "tags": [
          "s3"
        ],
        "summary": "Put object, copy object, or upload multipart part",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ObjectKey"
          },
          {
            "name": "uploadId",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Multipart upload ID when uploading a part."
          },
          {
            "name": "partNumber",
            "in": "query",
            "schema": {
              "type": "integer",
              "minimum": 1
            }
          },
          {
            "name": "x-amz-copy-source",
            "in": "header",
            "schema": {
              "type": "string"
            },
            "description": "Use /source-bucket/source-key to copy an existing object."
          }
        ],
        "requestBody": {
          "required": false,
          "content": {
            "application/octet-stream": {
              "schema": {
                "type": "string",
                "format": "binary"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Object written, copied, or part uploaded"
          },
          "400": {
            "$ref": "#/components/responses/S3Error"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          },
          "501": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      },
      "post": {
        "tags": [
          "s3"
        ],
        "summary": "Create or complete multipart upload",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ObjectKey"
          },
          {
            "name": "uploads",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to create a multipart upload."
          },
          {
            "name": "uploadId",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to complete a multipart upload."
          }
        ],
        "requestBody": {
          "required": false,
          "content": {
            "application/xml": {
              "schema": {
                "type": "string"
              }
            }
          }
        },
        "responses": {
          "200": {
            "$ref": "#/components/responses/S3XML"
          },
          "400": {
            "$ref": "#/components/responses/S3Error"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      },
      "delete": {
        "tags": [
          "s3"
        ],
        "summary": "Delete object or abort multipart upload",
        "security": [
          {
            "s3SigV4": []
          }
        ],
        "parameters": [
          {
            "$ref": "#/components/parameters/BucketKey"
          },
          {
            "$ref": "#/components/parameters/ObjectKey"
          },
          {
            "name": "uploadId",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "description": "Present to abort a multipart upload instead of deleting an object."
          }
        ],
        "responses": {
          "204": {
            "description": "Deleted"
          },
          "400": {
            "$ref": "#/components/responses/S3Error"
          },
          "404": {
            "$ref": "#/components/responses/S3Error"
          }
        }
      }
    }
  },
  "components": {
    "securitySchemes": {
      "basicAuth": {
        "type": "http",
        "scheme": "basic"
      },
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      },
      "s3SigV4": {
        "type": "apiKey",
        "in": "header",
        "name": "Authorization",
        "description": "AWS Signature Version 4 authorization header using bucket access credentials."
      }
    },
    "parameters": {
      "ID": {
        "name": "id",
        "in": "path",
        "required": true,
        "schema": {
          "type": "string"
        }
      },
      "FileID": {
        "name": "file_id",
        "in": "path",
        "required": true,
        "schema": {
          "type": "string"
        }
      },
      "GrantID": {
        "name": "grant_id",
        "in": "path",
        "required": true,
        "schema": {
          "type": "string"
        }
      },
      "BucketKey": {
        "name": "bucket",
        "in": "path",
        "required": true,
        "schema": {
          "type": "string"
        }
      },
      "ObjectKey": {
        "name": "objectKey",
        "in": "path",
        "required": true,
        "schema": {
          "type": "string"
        },
        "description": "Object key. The Gin route accepts slash-delimited keys."
      },
      "ListType": {
        "name": "list-type",
        "in": "query",
        "schema": {
          "type": "string",
          "enum": [
            "2"
          ]
        },
        "description": "Set to 2 for ListObjectsV2."
      },
      "Prefix": {
        "name": "prefix",
        "in": "query",
        "schema": {
          "type": "string"
        }
      },
      "Delimiter": {
        "name": "delimiter",
        "in": "query",
        "schema": {
          "type": "string"
        }
      },
      "MaxKeys": {
        "name": "max-keys",
        "in": "query",
        "schema": {
          "type": "integer",
          "minimum": 0,
          "default": 1000
        }
      },
      "ContinuationToken": {
        "name": "continuation-token",
        "in": "query",
        "schema": {
          "type": "string"
        }
      },
      "StartAfter": {
        "name": "start-after",
        "in": "query",
        "schema": {
          "type": "string"
        }
      },
      "Range": {
        "name": "Range",
        "in": "header",
        "schema": {
          "type": "string",
          "example": "bytes=0-1023"
        }
      }
    },
    "responses": {
      "PlainOK": {
        "description": "OK",
        "content": {
          "text/plain": {
            "schema": {
              "type": "string"
            }
          }
        }
      },
      "PlainError": {
        "description": "Error",
        "content": {
          "text/plain": {
            "schema": {
              "type": "string"
            }
          }
        }
      },
      "JSONError": {
        "description": "Error",
        "content": {
          "application/json": {
            "schema": {
              "$ref": "#/components/schemas/Error"
            }
          }
        }
      },
      "BinaryFile": {
        "description": "Binary file content",
        "content": {
          "application/octet-stream": {
            "schema": {
              "type": "string",
              "format": "binary"
            }
          }
        }
      },
      "BinaryOrS3XML": {
        "description": "Binary object content or S3 XML response",
        "content": {
          "application/octet-stream": {
            "schema": {
              "type": "string",
              "format": "binary"
            }
          },
          "application/xml": {
            "schema": {
              "type": "string"
            }
          }
        }
      },
      "S3XML": {
        "description": "S3 XML response",
        "content": {
          "application/xml": {
            "schema": {
              "type": "string"
            }
          }
        }
      },
      "S3Error": {
        "description": "S3 XML error",
        "content": {
          "application/xml": {
            "schema": {
              "$ref": "#/components/schemas/S3Error"
            }
          }
        }
      }
    },
    "schemas": {
      "Error": {
        "type": "object",
        "properties": {
          "error": {
            "type": "string"
          }
        }
      },
      "S3Error": {
        "type": "object",
        "xml": {
          "name": "Error"
        },
        "properties": {
          "Code": {
            "type": "string"
          },
          "Message": {
            "type": "string"
          }
        }
      },
      "CreateUserRequest": {
        "type": "object",
        "required": [
          "username"
        ],
        "properties": {
          "username": {
            "type": "string"
          }
        }
      },
      "CreateUserResponse": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          },
          "password": {
            "type": "string"
          }
        }
      },
      "CurrentUser": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "username": {
            "type": "string"
          },
          "avatar_url": {
            "type": "string"
          },
          "github_id": {
            "type": "integer",
            "format": "int64"
          }
        }
      },
      "TokenResponse": {
        "type": "object",
        "properties": {
          "token": {
            "type": "string"
          }
        }
      },
      "CreateBucketRequest": {
        "type": "object",
        "required": [
          "key",
          "source_ids"
        ],
        "properties": {
          "key": {
            "type": "string"
          },
          "source_ids": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "minItems": 1
          },
          "distribution_strategy": {
            "type": "string",
            "enum": [
              "round_robin",
              "weighted"
            ]
          },
          "source_weights": {
            "type": "object",
            "additionalProperties": {
              "type": "integer"
            }
          }
        }
      },
      "CreateBucketResponse": {
        "type": "object",
        "properties": {
          "key": {
            "type": "string"
          },
          "access_key": {
            "type": "string"
          },
          "access_secret": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          }
        }
      },
      "Bucket": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "key": {
            "type": "string"
          },
          "access_key": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          },
          "role": {
            "type": "string",
            "enum": [
              "owner",
              "editor",
              "viewer"
            ]
          },
          "shared": {
            "type": "boolean"
          }
        }
      },
      "UpdateBucketDistributionRequest": {
        "type": "object",
        "properties": {
          "distribution_strategy": {
            "type": "string",
            "enum": [
              "round_robin",
              "weighted"
            ]
          },
          "source_weights": {
            "type": "object",
            "additionalProperties": {
              "type": "integer"
            }
          }
        }
      },
      "CreateGrantRequest": {
        "type": "object",
        "required": [
          "username",
          "role"
        ],
        "properties": {
          "username": {
            "type": "string"
          },
          "role": {
            "type": "string",
            "enum": [
              "owner",
              "editor",
              "viewer"
            ]
          }
        }
      },
      "UpdateGrantRequest": {
        "type": "object",
        "required": [
          "role"
        ],
        "properties": {
          "role": {
            "type": "string",
            "enum": [
              "owner",
              "editor",
              "viewer"
            ]
          }
        }
      },
      "Grant": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "bucket_id": {
            "type": "string"
          },
          "user_id": {
            "type": "string"
          },
          "username": {
            "type": "string"
          },
          "role": {
            "type": "string"
          },
          "granted_by": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          }
        }
      },
      "FileUploadRequest": {
        "type": "object",
        "required": [
          "file"
        ],
        "properties": {
          "file": {
            "type": "string",
            "format": "binary"
          }
        }
      },
      "File": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          },
          "size": {
            "type": "integer",
            "format": "int64"
          }
        }
      },
      "CreateShareRequest": {
        "type": "object",
        "properties": {
          "expires_in": {
            "type": "integer",
            "description": "Seconds until expiry."
          }
        }
      },
      "ShareLink": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "file_id": {
            "type": "string"
          },
          "file_name": {
            "type": "string"
          },
          "token": {
            "type": "string"
          },
          "url": {
            "type": "string"
          },
          "expires_at": {
            "type": "string",
            "format": "date-time"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          }
        }
      },
      "Source": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "type": {
            "type": "string",
            "enum": [
              "gdrive",
              "telegram",
              "s3"
            ]
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          }
        }
      },
      "CreateGDriveSourceRequest": {
        "type": "object",
        "required": [
          "name",
          "key"
        ],
        "properties": {
          "name": {
            "type": "string"
          },
          "key": {
            "type": "string"
          }
        }
      },
      "CreateTelegramSourceRequest": {
        "type": "object",
        "required": [
          "name",
          "token",
          "chat_id"
        ],
        "properties": {
          "name": {
            "type": "string"
          },
          "token": {
            "type": "string"
          },
          "chat_id": {
            "type": "string"
          }
        }
      },
      "CreateS3SourceRequest": {
        "type": "object",
        "required": [
          "name",
          "endpoint",
          "bucket",
          "access_key_id",
          "secret_access_key"
        ],
        "properties": {
          "name": {
            "type": "string"
          },
          "endpoint": {
            "type": "string"
          },
          "bucket": {
            "type": "string"
          },
          "access_key_id": {
            "type": "string"
          },
          "secret_access_key": {
            "type": "string"
          },
          "region": {
            "type": "string"
          },
          "path_style": {
            "type": "boolean"
          }
        }
      },
      "SourceInfo": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "type": {
            "type": "string"
          },
          "files": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/SourceInfoFile"
            }
          },
          "storage_total": {
            "type": "integer",
            "format": "int64"
          },
          "storage_used": {
            "type": "integer",
            "format": "int64"
          },
          "storage_free": {
            "type": "integer",
            "format": "int64"
          }
        }
      },
      "SourceInfoFile": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "size": {
            "type": "integer",
            "format": "int64"
          }
        }
      }
    }
  }
}`

func OpenAPIJSON() []byte {
	return []byte(openAPIJSON)
}
