package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const requestTimeout = 20 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-helper: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return usage()
	}

	switch args[0] {
	case "ready":
		if len(args) != 2 {
			return usage()
		}
		return ready(args[1])
	case "create-user":
		if len(args) != 3 {
			return usage()
		}
		password, err := createUser(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Println(password)
	case "create-source":
		if len(args) != 5 {
			return usage()
		}
		sourceID, err := createSource(args[1], args[2], args[3], args[4])
		if err != nil {
			return err
		}
		fmt.Println(sourceID)
	case "share-url":
		if len(args) != 6 {
			return usage()
		}
		shareURL, err := createShare(args[1], args[2], args[3], args[4], args[5])
		if err != nil {
			return err
		}
		fmt.Println(shareURL)
	case "download-url":
		if len(args) != 3 {
			return usage()
		}
		return downloadURL(args[1], args[2])
	case "s3-get":
		if len(args) != 7 {
			return usage()
		}
		return s3Get(args[1], args[2], args[3], args[4], args[5], args[6])
	default:
		return usage()
	}

	return nil
}

func usage() error {
	return errors.New("usage: smoke-helper ready URL | create-user BASE_URL USERNAME | create-source BASE_URL USERNAME PASSWORD SOURCE_NAME | share-url BASE_URL USERNAME PASSWORD BUCKET_ID FILE_ID | download-url URL OUTPUT | s3-get ENDPOINT ACCESS_KEY SECRET_KEY BUCKET KEY OUTPUT")
}

func ready(rawURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %s", rawURL, resp.Status)
	}
	return nil
}

func createUser(baseURL, username string) (string, error) {
	var out struct {
		Password string `json:"password"`
	}
	err := postJSON(baseURL+"/api/v1/users", "", "", map[string]string{"username": username}, &out)
	if err != nil {
		return "", err
	}
	if out.Password == "" {
		return "", errors.New("user creation response did not include a password")
	}
	return out.Password, nil
}

func createSource(baseURL, username, password, sourceName string) (string, error) {
	payload := map[string]any{
		"name":              sourceName,
		"endpoint":          "http://minio:9000",
		"bucket":            "sfree-data",
		"access_key_id":     "minioadmin",
		"secret_access_key": "minioadmin",
		"region":            "us-east-1",
		"path_style":        true,
	}
	var out struct {
		ID string `json:"id"`
	}
	err := postJSON(baseURL+"/api/v1/sources/s3", username, password, payload, &out)
	if err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("S3 source creation response did not include an id")
	}
	return out.ID, nil
}

func createShare(baseURL, username, password, bucketID, fileID string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := postJSON(baseURL+"/api/v1/buckets/"+bucketID+"/files/"+fileID+"/share", username, password, map[string]any{}, &out)
	if err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errors.New("share creation response did not include a URL")
	}
	return out.URL, nil
}

func postJSON(rawURL, username, password string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("POST %s returned %s: %s", rawURL, resp.Status, string(responseBody))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func downloadURL(rawURL, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s returned %s: %s", rawURL, resp.Status, string(responseBody))
	}
	return writeFile(output, resp.Body)
}

func s3Get(endpoint, accessKey, secretKey, bucket, key, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return err
	}
	client := awss3.NewFromConfig(cfg, func(opts *awss3.Options) {
		opts.BaseEndpoint = aws.String(endpoint)
		opts.UsePathStyle = true
	})
	result, err := client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()
	return writeFile(output, result.Body)
}

func writeFile(output string, source io.Reader) error {
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, source); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
