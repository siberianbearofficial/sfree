package s3compat

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestParseConfigRequiresFields(t *testing.T) {
	t.Parallel()
	_, err := ParseConfig(`{"endpoint":"http://localhost:9000"}`)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseConfigSetsDefaultRegion(t *testing.T) {
	t.Parallel()
	cfg, err := ParseConfig(`{"endpoint":"http://localhost:9000","bucket":"b","access_key_id":"a","secret_access_key":"s"}`)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("unexpected region: %s", cfg.Region)
	}
}

func TestListObjectsFollowsPagination(t *testing.T) {
	t.Parallel()
	cli := &Client{cfg: Config{Bucket: "bucket"}}
	calls := 0
	cli.listObjectsV2 = func(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		calls++
		switch calls {
		case 1:
			if input.ContinuationToken != nil {
				t.Fatalf("unexpected continuation token on first call: %v", *input.ContinuationToken)
			}
			return &s3.ListObjectsV2Output{
				Contents:              []types.Object{{Key: aws.String("a"), Size: aws.Int64(3)}},
				IsTruncated:           aws.Bool(true),
				NextContinuationToken: aws.String("next-token"),
			}, nil
		case 2:
			if aws.ToString(input.ContinuationToken) != "next-token" {
				t.Fatalf("unexpected continuation token: %s", aws.ToString(input.ContinuationToken))
			}
			return &s3.ListObjectsV2Output{
				Contents:    []types.Object{{Key: aws.String("b"), Size: aws.Int64(7)}},
				IsTruncated: aws.Bool(false),
			}, nil
		default:
			t.Fatalf("unexpected extra call: %d", calls)
			return nil, nil
		}
	}

	objects, total, err := cli.ListObjects(context.Background())
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if calls != 2 {
		t.Fatalf("unexpected calls count: %d", calls)
	}
	if len(objects) != 2 {
		t.Fatalf("unexpected object count: %d", len(objects))
	}
	if objects[0].Key != "a" || objects[1].Key != "b" {
		t.Fatalf("unexpected keys: %+v", objects)
	}
	if total != 10 {
		t.Fatalf("unexpected total: %d", total)
	}
}

func TestHeadBucketUsesBucketMetadataProbe(t *testing.T) {
	t.Parallel()
	cli := &Client{cfg: Config{Bucket: "bucket"}}
	calls := 0
	cli.headBucket = func(_ context.Context, input *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
		calls++
		if aws.ToString(input.Bucket) != "bucket" {
			t.Fatalf("unexpected bucket: %s", aws.ToString(input.Bucket))
		}
		return &s3.HeadBucketOutput{}, nil
	}
	cli.listObjectsV2 = func(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		t.Fatal("health probe must not list objects")
		return nil, nil
	}

	if err := cli.HeadBucket(context.Background()); err != nil {
		t.Fatalf("head bucket: %v", err)
	}
	if calls != 1 {
		t.Fatalf("unexpected calls count: %d", calls)
	}
}
