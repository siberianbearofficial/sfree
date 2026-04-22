package manager

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3compat"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type healthTestClient struct {
	storageTotal int64
	storageUsed  int64
	storageFree  int64
	storageErr   error
	headErr      error
	chatErr      error
	headCalls    int
	chatCalls    int
	listCalls    int
}

func (c *healthTestClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c *healthTestClient) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (c *healthTestClient) Delete(context.Context, string) error {
	return nil
}

func (c *healthTestClient) StorageInfo(context.Context) (int64, int64, int64, error) {
	return c.storageTotal, c.storageUsed, c.storageFree, c.storageErr
}

func (c *healthTestClient) HeadBucket(context.Context) error {
	c.headCalls++
	return c.headErr
}

func (c *healthTestClient) CheckChat(context.Context) error {
	c.chatCalls++
	return c.chatErr
}

func (c *healthTestClient) ListObjects(context.Context) ([]s3compat.ObjectInfo, int64, error) {
	c.listCalls++
	return nil, 0, nil
}

func TestCheckSourceHealthGDriveReturnsQuota(t *testing.T) {
	t.Parallel()
	cli := &healthTestClient{storageTotal: 1000, storageUsed: 400, storageFree: 600}
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeGDrive}

	health, err := CheckSourceHealth(context.Background(), src, func(context.Context, *repository.Source) (SourceClient, error) {
		return cli, nil
	})
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if health.Status != SourceHealthHealthy || health.ReasonCode != "ok" {
		t.Fatalf("unexpected health: %+v", health)
	}
	if health.Quota.TotalBytes == nil || *health.Quota.TotalBytes != 1000 {
		t.Fatalf("expected quota total, got %+v", health.Quota)
	}
}

func TestCheckSourceHealthGDriveLowQuotaIsDegraded(t *testing.T) {
	t.Parallel()
	cli := &healthTestClient{storageTotal: 1000, storageUsed: 970, storageFree: 30}
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeGDrive}

	health, err := CheckSourceHealth(context.Background(), src, func(context.Context, *repository.Source) (SourceClient, error) {
		return cli, nil
	})
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if health.Status != SourceHealthDegraded || health.ReasonCode != "quota_low" {
		t.Fatalf("unexpected health: %+v", health)
	}
}

func TestCheckSourceHealthS3UsesBucketMetadataProbe(t *testing.T) {
	t.Parallel()
	cli := &healthTestClient{}
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeS3}

	health, err := CheckSourceHealth(context.Background(), src, func(context.Context, *repository.Source) (SourceClient, error) {
		return cli, nil
	})
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if health.Status != SourceHealthHealthy {
		t.Fatalf("unexpected health: %+v", health)
	}
	if cli.headCalls != 1 {
		t.Fatalf("expected one head bucket call, got %d", cli.headCalls)
	}
	if cli.listCalls != 0 {
		t.Fatalf("health check must not list objects, got %d calls", cli.listCalls)
	}
	if health.Quota.TotalBytes != nil || health.Quota.UsedBytes != nil || health.Quota.FreeBytes != nil {
		t.Fatalf("expected unknown quota for s3, got %+v", health.Quota)
	}
}

func TestCheckSourceHealthTelegramFailureIsUnhealthy(t *testing.T) {
	t.Parallel()
	cli := &healthTestClient{chatErr: errors.New("chat not found")}
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeTelegram}

	health, err := CheckSourceHealth(context.Background(), src, func(context.Context, *repository.Source) (SourceClient, error) {
		return cli, nil
	})
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if health.Status != SourceHealthUnhealthy || health.ReasonCode != "probe_failed" {
		t.Fatalf("unexpected health: %+v", health)
	}
	if cli.chatCalls != 1 {
		t.Fatalf("expected one chat check, got %d", cli.chatCalls)
	}
}

func TestCheckSourceHealthClientCreationFailureIsUnhealthy(t *testing.T) {
	t.Parallel()
	src := &repository.Source{ID: primitive.NewObjectID(), Type: repository.SourceTypeS3}

	health, err := CheckSourceHealth(context.Background(), src, func(context.Context, *repository.Source) (SourceClient, error) {
		return nil, errors.New("bad config")
	})
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if health.Status != SourceHealthUnhealthy || health.ReasonCode != "client_error" {
		t.Fatalf("unexpected health: %+v", health)
	}
}
