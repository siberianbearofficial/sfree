package handlers

import (
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

const (
	maxDeleteObjects                = 1000
	maxDeleteObjectsRequestBodySize = 8 * 1024 * 1024
)

var (
	errDeleteObjectsMalformedXML = errors.New("malformed delete objects XML")
	errDeleteObjectsTooMany      = errors.New("too many delete objects")
)

type deleteObjectsRequest struct {
	XMLName xml.Name              `xml:"Delete"`
	Quiet   bool                  `xml:"Quiet"`
	Objects []deleteObjectRequest `xml:"Object"`
}

type deleteObjectRequest struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId,omitempty"`
}

type deleteObjectsResult struct {
	XMLName xml.Name              `xml:"DeleteResult"`
	Xmlns   string                `xml:"xmlns,attr"`
	Deleted []deletedObjectResult `xml:"Deleted,omitempty"`
	Errors  []deleteObjectError   `xml:"Error,omitempty"`
}

type deletedObjectResult struct {
	Key string `xml:"Key"`
}

type deleteObjectError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func decodeDeleteObjectsRequest(r io.Reader) (deleteObjectsRequest, error) {
	decoder := xml.NewDecoder(r)
	var req deleteObjectsRequest
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return req, errDeleteObjectsMalformedXML
			}
			return req, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "Delete" {
			return req, errDeleteObjectsMalformedXML
		}
		if err := decodeDeleteObjectsElement(decoder, &req); err != nil {
			return req, err
		}
		return req, nil
	}
}

func decodeDeleteObjectsElement(decoder *xml.Decoder, req *deleteObjectsRequest) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errDeleteObjectsMalformedXML
			}
			return err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "Quiet":
				if err := decoder.DecodeElement(&req.Quiet, &tok); err != nil {
					return err
				}
			case "Object":
				if len(req.Objects) >= maxDeleteObjects {
					return errDeleteObjectsTooMany
				}
				obj, err := decodeDeleteObjectElement(decoder)
				if err != nil {
					return err
				}
				req.Objects = append(req.Objects, obj)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == "Delete" {
				return nil
			}
		}
	}
}

func decodeDeleteObjectElement(decoder *xml.Decoder) (deleteObjectRequest, error) {
	var obj deleteObjectRequest
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return obj, errDeleteObjectsMalformedXML
			}
			return obj, err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "Key":
				if err := decoder.DecodeElement(&obj.Key, &tok); err != nil {
					return obj, err
				}
			case "VersionId":
				if err := decoder.DecodeElement(&obj.VersionID, &tok); err != nil {
					return obj, err
				}
			default:
				if err := decoder.Skip(); err != nil {
					return obj, err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == "Object" {
				return obj, nil
			}
		}
	}
}

func DeleteObjects(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return DeleteObjectsWithFactory(bucketRepo, sourceRepo, fileRepo, nil)
}

func DeleteObjectsWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectServiceWithSourceClientFactory(sourceRepo, fileRepo, nil, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}

		req, err := decodeDeleteObjectsRequest(http.MaxBytesReader(c.Writer, c.Request.Body, maxDeleteObjectsRequestBodySize))
		if err != nil {
			if errors.Is(err, errDeleteObjectsTooMany) {
				writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "DeleteObjects supports at most 1000 objects per request")
				return
			}
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "DeleteObjects request body is too large")
				return
			}
			writeS3Error(c, http.StatusBadRequest, "MalformedXML", "The XML you provided was not well-formed or did not validate against our published schema")
			return
		}

		result := deleteObjectsResult{
			Xmlns:   "http://s3.amazonaws.com/doc/2006-03-01/",
			Deleted: make([]deletedObjectResult, 0, len(req.Objects)),
			Errors:  make([]deleteObjectError, 0),
		}
		for _, obj := range req.Objects {
			key := obj.Key
			if key == "" {
				result.Errors = append(result.Errors, deleteObjectError{
					Key:     key,
					Code:    "InvalidArgument",
					Message: "Object key must not be empty",
				})
				continue
			}
			deleteResult, err := objectSvc.DeleteObject(ctx, bucketDoc.ID, key)
			if err != nil {
				slog.ErrorContext(ctx, "delete objects: delete file", slog.String("key", key), slog.String("error", err.Error()))
				result.Errors = append(result.Errors, deleteObjectError{
					Key:     key,
					Code:    "InternalError",
					Message: "Internal error deleting object",
				})
				continue
			}
			if deleteResult.CleanupErr != nil {
				slog.WarnContext(ctx, "delete objects: delete chunks", slog.String("key", key), slog.String("error", deleteResult.CleanupErr.Error()))
			}
			if !req.Quiet {
				result.Deleted = append(result.Deleted, deletedObjectResult{Key: key})
			}
		}
		c.XML(http.StatusOK, result)
	}
}
