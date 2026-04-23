package manager

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/sourcecap"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type healthTestClient struct {
	health     sourcecap.Health
	healthErr  error
	probeCalls int
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

func (c *healthTestClient) ProbeSourceHealth(context.Context) (sourcecap.Health, error) {
	c.probeCalls++
	return c.health, c.healthErr
}

func TestCheckSourceHealthGDriveReturnsQuota(t *testing.T) {
	t.Parallel()
	total, used, free := int64(1000), int64(400), int64(600)
	cli := &healthTestClient{health: sourcecap.Health{
		Status:     sourcecap.HealthHealthy,
		ReasonCode: "ok",
		Message:    "Google Drive metadata is reachable.",
		Quota:      sourcecap.Quota{TotalBytes: &total, UsedBytes: &used, FreeBytes: &free},
	}}
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
	cli := &healthTestClient{health: sourcecap.Health{
		Status:     sourcecap.HealthDegraded,
		ReasonCode: "quota_low",
		Message:    "Google Drive quota is nearly exhausted.",
	}}
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
	cli := &healthTestClient{health: sourcecap.Health{
		Status:     sourcecap.HealthHealthy,
		ReasonCode: "ok",
		Message:    "S3 bucket metadata is reachable.",
	}}
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
	if cli.probeCalls != 1 {
		t.Fatalf("expected one health probe call, got %d", cli.probeCalls)
	}
	if health.Quota.TotalBytes != nil || health.Quota.UsedBytes != nil || health.Quota.FreeBytes != nil {
		t.Fatalf("expected unknown quota for s3, got %+v", health.Quota)
	}
}

func TestCheckSourceHealthTelegramFailureIsUnhealthy(t *testing.T) {
	t.Parallel()
	cli := &healthTestClient{
		health: sourcecap.Health{
			Status:     sourcecap.HealthUnhealthy,
			ReasonCode: "probe_failed",
			Message:    "Telegram bot or chat is not reachable.",
		},
		healthErr: errors.New("chat not found"),
	}
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
	if cli.probeCalls != 1 {
		t.Fatalf("expected one health probe call, got %d", cli.probeCalls)
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
