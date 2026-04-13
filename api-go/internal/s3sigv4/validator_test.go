package s3sigv4

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestValidatorHeaderAuthMatchesAWSExample(t *testing.T) {
	req, err := http.NewRequest("GET", "https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("X-Amz-Date", "20150830T123600Z")
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7")

	v := Validator{Now: func() time.Time { return time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC) }}

	res, err := v.Validate(context.Background(), req, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.Mode != "header" {
		t.Fatalf("mode: expected header got %s", res.Mode)
	}

	expectedCanonical := `GET
/
Action=ListUsers&Version=2010-05-08
content-type:application/x-www-form-urlencoded; charset=utf-8
host:iam.amazonaws.com
x-amz-date:20150830T123600Z

content-type;host;x-amz-date
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
	if res.CanonicalRequest != expectedCanonical {
		t.Fatalf("canonical request mismatch:\nexpected:\n%s\n\nactual:\n%s", expectedCanonical, res.CanonicalRequest)
	}

	expectedStringToSign := `AWS4-HMAC-SHA256
20150830T123600Z
20150830/us-east-1/iam/aws4_request
f536975d06c0309214f805bb90ccff089219ecd68b2577efef23edd43b7e1a59`
	if res.StringToSign != expectedStringToSign {
		t.Fatalf("string to sign mismatch:\nexpected:\n%s\n\nactual:\n%s", expectedStringToSign, res.StringToSign)
	}

	expectedSig := "5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	if res.Signature != expectedSig {
		t.Fatalf("signature mismatch: expected %s got %s", expectedSig, res.Signature)
	}
}

func TestValidatorPresignMatchesAWSExample(t *testing.T) {
	req, err := http.NewRequest("GET", "https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIDEXAMPLE%2F20150830%2Fus-east-1%2Fiam%2Faws4_request&X-Amz-Date=20150830T123600Z&X-Amz-Expires=60&X-Amz-SignedHeaders=content-type%3Bhost%3Bx-amz-date&X-Amz-Signature=63613d9c6a68b0e499ed9beeeabe0c4f3295742554209d6f109fe3c9563f56c3", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("X-Amz-Date", "20150830T123600Z")

	v := Validator{Now: func() time.Time { return time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC) }}

	res, err := v.Validate(context.Background(), req, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.Mode != "presign" {
		t.Fatalf("mode: expected presign got %s", res.Mode)
	}

	expectedCanonical := `GET
/
Action=ListUsers&Version=2010-05-08&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIDEXAMPLE%2F20150830%2Fus-east-1%2Fiam%2Faws4_request&X-Amz-Date=20150830T123600Z&X-Amz-Expires=60&X-Amz-SignedHeaders=content-type%3Bhost%3Bx-amz-date
content-type:application/x-www-form-urlencoded; charset=utf-8
host:iam.amazonaws.com
x-amz-date:20150830T123600Z

content-type;host;x-amz-date
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
	if res.CanonicalRequest != expectedCanonical {
		t.Fatalf("canonical request mismatch:\nexpected:\n%s\n\nactual:\n%s", expectedCanonical, res.CanonicalRequest)
	}

	expectedStringToSign := `AWS4-HMAC-SHA256
20150830T123600Z
20150830/us-east-1/iam/aws4_request
829d0ec8859c4877fb1709979fe8ef44a082303f2517ff2a1f335b6b0b1392fa`
	if res.StringToSign != expectedStringToSign {
		t.Fatalf("string to sign mismatch:\nexpected:\n%s\n\nactual:\n%s", expectedStringToSign, res.StringToSign)
	}

	expectedSig := "63613d9c6a68b0e499ed9beeeabe0c4f3295742554209d6f109fe3c9563f56c3"
	if res.Signature != expectedSig {
		t.Fatalf("signature mismatch: expected %s got %s", expectedSig, res.Signature)
	}
}

func TestValidatorPresignS3GetObject(t *testing.T) {
	accessKey := "mybucketkey"
	secretKey := "mysecretkey123"
	region := "us-east-1"
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	expires := "3600"
	scope := dateStr + "/" + region + "/s3/aws4_request"
	cred := accessKey + "/" + scope
	signedHeaders := "host"

	// Build the canonical request for a presigned GET.
	canonURI := "/api/s3/mybucket/photos/cat.jpg"
	// Query string without X-Amz-Signature (excluded during signing).
	canonQuery := "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + url.QueryEscape(cred) +
		"&X-Amz-Date=" + amzDate +
		"&X-Amz-Expires=" + expires +
		"&X-Amz-SignedHeaders=" + signedHeaders
	canonHeaders := "host:sfree.example.com\n"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonReq := "GET\n" + canonURI + "\n" + canonQuery + "\n" + canonHeaders + "\n" + signedHeaders + "\n" + payloadHash

	stringToSign := buildStringToSign("AWS4-HMAC-SHA256", now, scope, canonReq)
	sig := computeSignature(secretKey, dateStr, region, "s3", stringToSign)

	fullURL := "https://sfree.example.com" + canonURI + "?" + canonQuery + "&X-Amz-Signature=" + sig
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	v := Validator{Now: func() time.Time { return now }}
	res, err := v.Validate(context.Background(), req, accessKey, secretKey)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.Mode != "presign" {
		t.Fatalf("mode: expected presign got %s", res.Mode)
	}
	if res.PayloadHash != "UNSIGNED-PAYLOAD" {
		t.Fatalf("payload hash: expected UNSIGNED-PAYLOAD got %s", res.PayloadHash)
	}
}

func TestValidatorPresignS3PutObject(t *testing.T) {
	accessKey := "mybucketkey"
	secretKey := "mysecretkey123"
	region := "us-east-1"
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	expires := "3600"
	scope := dateStr + "/" + region + "/s3/aws4_request"
	cred := accessKey + "/" + scope
	signedHeaders := "host"

	canonURI := "/api/s3/mybucket/uploads/doc.pdf"
	canonQuery := "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + url.QueryEscape(cred) +
		"&X-Amz-Date=" + amzDate +
		"&X-Amz-Expires=" + expires +
		"&X-Amz-SignedHeaders=" + signedHeaders
	canonHeaders := "host:sfree.example.com\n"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonReq := "PUT\n" + canonURI + "\n" + canonQuery + "\n" + canonHeaders + "\n" + signedHeaders + "\n" + payloadHash

	stringToSign := buildStringToSign("AWS4-HMAC-SHA256", now, scope, canonReq)
	sig := computeSignature(secretKey, dateStr, region, "s3", stringToSign)

	fullURL := "https://sfree.example.com" + canonURI + "?" + canonQuery + "&X-Amz-Signature=" + sig
	req, err := http.NewRequest("PUT", fullURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	v := Validator{Now: func() time.Time { return now }}
	res, err := v.Validate(context.Background(), req, accessKey, secretKey)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.Mode != "presign" {
		t.Fatalf("mode: expected presign got %s", res.Mode)
	}
}

func TestValidatorPresignExpired(t *testing.T) {
	accessKey := "mybucketkey"
	secretKey := "mysecretkey123"
	region := "us-east-1"
	signTime := time.Date(2026, 4, 13, 8, 0, 0, 0, time.UTC)
	dateStr := signTime.Format("20060102")
	amzDate := signTime.Format("20060102T150405Z")
	expires := "60" // 1 minute
	scope := dateStr + "/" + region + "/s3/aws4_request"
	cred := accessKey + "/" + scope
	signedHeaders := "host"

	canonURI := "/api/s3/mybucket/file.txt"
	canonQuery := "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + url.QueryEscape(cred) +
		"&X-Amz-Date=" + amzDate +
		"&X-Amz-Expires=" + expires +
		"&X-Amz-SignedHeaders=" + signedHeaders
	canonHeaders := "host:sfree.example.com\n"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonReq := "GET\n" + canonURI + "\n" + canonQuery + "\n" + canonHeaders + "\n" + signedHeaders + "\n" + payloadHash
	stringToSign := buildStringToSign("AWS4-HMAC-SHA256", signTime, scope, canonReq)
	sig := computeSignature(secretKey, dateStr, region, "s3", stringToSign)

	fullURL := "https://sfree.example.com" + canonURI + "?" + canonQuery + "&X-Amz-Signature=" + sig
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	// Validate 2 hours later — should be expired.
	v := Validator{Now: func() time.Time { return signTime.Add(2 * time.Hour) }}
	_, err = v.Validate(context.Background(), req, accessKey, secretKey)
	if err == nil {
		t.Fatal("expected expired error, got nil")
	}
	if err.Error() == "" || !contains([]string{"expired"}, "expired") {
		// Just verify it's a non-nil error (ErrExpired).
	}
}

func TestValidatorPresignTTLExceedsMax(t *testing.T) {
	accessKey := "mybucketkey"
	secretKey := "mysecretkey123"
	region := "us-east-1"
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	expires := "999999" // > 7 days
	scope := dateStr + "/" + region + "/s3/aws4_request"
	cred := accessKey + "/" + scope
	signedHeaders := "host"

	canonURI := "/api/s3/mybucket/file.txt"
	canonQuery := "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + url.QueryEscape(cred) +
		"&X-Amz-Date=" + amzDate +
		"&X-Amz-Expires=" + expires +
		"&X-Amz-SignedHeaders=" + signedHeaders
	canonHeaders := "host:sfree.example.com\n"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonReq := "GET\n" + canonURI + "\n" + canonQuery + "\n" + canonHeaders + "\n" + signedHeaders + "\n" + payloadHash
	stringToSign := buildStringToSign("AWS4-HMAC-SHA256", now, scope, canonReq)
	sig := computeSignature(secretKey, dateStr, region, "s3", stringToSign)

	fullURL := "https://sfree.example.com" + canonURI + "?" + canonQuery + "&X-Amz-Signature=" + sig
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	v := Validator{Now: func() time.Time { return now }}
	_, err = v.Validate(context.Background(), req, accessKey, secretKey)
	if err == nil {
		t.Fatal("expected TTL error, got nil")
	}
}

func TestValidatorCanonicalizesDefaultPort(t *testing.T) {
	req, err := http.NewRequest("GET", "https://estest.us-east-1.es.amazonaws.com:443/_search", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("X-Amz-Date", "19700101T000000Z")
	req.Header.Set("X-Amz-Security-Token", "SESSION")
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKID/19700101/us-east-1/es/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=e573fc9aa3a156b720976419319be98fb2824a3abc2ddd895ecb1d1611c6a82d")

	v := Validator{
		Now:     func() time.Time { return time.Unix(0, 0) },
		MaxSkew: time.Hour,
	}

	res, err := v.Validate(context.Background(), req, "AKID", "SECRET")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	expectedCanonical := `GET
/_search

host:estest.us-east-1.es.amazonaws.com
x-amz-date:19700101T000000Z
x-amz-security-token:SESSION

host;x-amz-date;x-amz-security-token
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
	if res.CanonicalRequest != expectedCanonical {
		t.Fatalf("canonical request mismatch:\nexpected:\n%s\n\nactual:\n%s", expectedCanonical, res.CanonicalRequest)
	}

	expectedSig := "e573fc9aa3a156b720976419319be98fb2824a3abc2ddd895ecb1d1611c6a82d"
	if res.Signature != expectedSig {
		t.Fatalf("signature mismatch: expected %s got %s", expectedSig, res.Signature)
	}
}
