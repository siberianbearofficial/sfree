package s3compat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	AccessKeyID  string `json:"access_key_id"`
	SecretAccess string `json:"secret_access_key"`
	PathStyle    bool   `json:"path_style"`
}

type ObjectInfo struct {
	Key  string
	Size int64
}

type Client struct {
	cfg Config
	s3  *s3.Client
}

func EncodeConfig(cfg Config) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseConfig(raw string) (Config, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Endpoint == "" {
		return Config{}, fmt.Errorf("s3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return Config{}, fmt.Errorf("s3 bucket is required")
	}
	if cfg.AccessKeyID == "" {
		return Config{}, fmt.Errorf("s3 access_key_id is required")
	}
	if cfg.SecretAccess == "" {
		return Config{}, fmt.Errorf("s3 secret_access_key is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg, nil
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccess, "")),
	)
	if err != nil {
		return nil, err
	}
	s3Client := s3.NewFromConfig(awsCfg, func(opts *s3.Options) {
		opts.BaseEndpoint = aws.String(cfg.Endpoint)
		opts.UsePathStyle = cfg.PathStyle
	})
	return &Client{cfg: cfg, s3: s3Client}, nil
}

func (c *Client) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(name), Body: r})
	if err != nil {
		return "", err
	}
	return name, nil
}

func (c *Client) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	obj, err := c.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(name)})
	if err != nil {
		return nil, err
	}
	return obj.Body, nil
}

func (c *Client) Delete(ctx context.Context, name string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(name)})
	return err
}

func (c *Client) ListObjects(ctx context.Context) ([]ObjectInfo, int64, error) {
	resp, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(c.cfg.Bucket)})
	if err != nil {
		return nil, 0, err
	}
	objects := make([]ObjectInfo, 0, len(resp.Contents))
	var total int64
	for _, obj := range resp.Contents {
		if obj.Key == nil {
			continue
		}
		sz := aws.ToInt64(obj.Size)
		objects = append(objects, ObjectInfo{Key: *obj.Key, Size: sz})
		total += sz
	}
	return objects, total, nil
}
