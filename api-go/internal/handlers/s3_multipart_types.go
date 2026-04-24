package handlers

import (
	"context"
	"encoding/xml"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type initiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

type completeMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type completeMultipartUploadRequest struct {
	XMLName xml.Name         `xml:"CompleteMultipartUpload"`
	Parts   []completionPart `xml:"Part"`
}

type completionPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type listMultipartUploadsResult struct {
	XMLName            xml.Name             `xml:"ListMultipartUploadsResult"`
	Xmlns              string               `xml:"xmlns,attr"`
	Bucket             string               `xml:"Bucket"`
	KeyMarker          string               `xml:"KeyMarker,omitempty"`
	UploadIDMarker     string               `xml:"UploadIdMarker,omitempty"`
	NextKeyMarker      string               `xml:"NextKeyMarker,omitempty"`
	NextUploadIDMarker string               `xml:"NextUploadIdMarker,omitempty"`
	Prefix             string               `xml:"Prefix,omitempty"`
	MaxUploads         int                  `xml:"MaxUploads"`
	Upload             []multipartUploadXML `xml:"Upload"`
	IsTruncated        bool                 `xml:"IsTruncated"`
}

type multipartUploadXML struct {
	Key       string `xml:"Key"`
	UploadId  string `xml:"UploadId"`
	Initiated string `xml:"Initiated"`
}

type listPartsResult struct {
	XMLName              xml.Name  `xml:"ListPartsResult"`
	Xmlns                string    `xml:"xmlns,attr"`
	Bucket               string    `xml:"Bucket"`
	Key                  string    `xml:"Key"`
	UploadId             string    `xml:"UploadId"`
	PartNumberMarker     int       `xml:"PartNumberMarker"`
	NextPartNumberMarker int       `xml:"NextPartNumberMarker,omitempty"`
	MaxParts             int       `xml:"MaxParts"`
	Part                 []partXML `xml:"Part"`
	IsTruncated          bool      `xml:"IsTruncated"`
}

type partXML struct {
	PartNumber   int    `xml:"PartNumber"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	LastModified string `xml:"LastModified"`
}

type multipartUploadAbortStore interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
	Delete(ctx context.Context, uploadID string) error
}

type multipartUploadPager interface {
	ListByBucketPage(ctx context.Context, bucketID primitive.ObjectID, prefix, keyMarker, uploadIDMarker string, limit int) ([]repository.MultipartUpload, bool, error)
}

type multipartUploadGetter interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
}

type multipartUploadListPage struct {
	entries            []multipartUploadXML
	isTruncated        bool
	nextKeyMarker      string
	nextUploadIDMarker string
}

type multipartPartsPage struct {
	parts                []partXML
	isTruncated          bool
	nextPartNumberMarker int
}
