package manager

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/sourcecap"
)

type SourceInfoFile struct {
	ID   string
	Name string
	Size int64
}

type SourceInfo struct {
	Files        []SourceInfoFile
	StorageTotal int64
	StorageUsed  int64
	StorageFree  int64
}

type SourceHealthStatus string

const (
	SourceHealthHealthy   SourceHealthStatus = "healthy"
	SourceHealthDegraded  SourceHealthStatus = "degraded"
	SourceHealthUnhealthy SourceHealthStatus = "unhealthy"
)

type SourceQuota struct {
	TotalBytes *int64
	UsedBytes  *int64
	FreeBytes  *int64
}

type SourceHealth struct {
	SourceID   string
	SourceType string
	Status     SourceHealthStatus
	CheckedAt  time.Time
	LatencyMS  int64
	ReasonCode string
	Message    string
	Quota      SourceQuota
}

func InspectSource(ctx context.Context, src *repository.Source, factory SourceClientFactory) (SourceInfo, error) {
	if src == nil {
		return SourceInfo{}, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive:
		cli, err := sourceClientFor(ctx, src, factory)
		if err != nil {
			return SourceInfo{}, err
		}
		infoClient, ok := cli.(sourcecap.InfoProvider)
		if !ok {
			return SourceInfo{}, ErrUnsupportedSourceType
		}
		info, err := infoClient.SourceInfo(ctx)
		if err != nil {
			return SourceInfo{}, err
		}
		respFiles := make([]SourceInfoFile, 0, len(info.Files))
		for _, f := range info.Files {
			respFiles = append(respFiles, SourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
		}
		return SourceInfo{Files: respFiles, StorageTotal: info.StorageTotal, StorageUsed: info.StorageUsed, StorageFree: info.StorageFree}, nil
	case repository.SourceTypeTelegram:
		return SourceInfo{Files: []SourceInfoFile{}}, nil
	case repository.SourceTypeS3:
		cli, err := sourceClientFor(ctx, src, factory)
		if err != nil {
			return SourceInfo{}, err
		}
		infoClient, ok := cli.(sourcecap.InfoProvider)
		if !ok {
			return SourceInfo{}, ErrUnsupportedSourceType
		}
		info, err := infoClient.SourceInfo(ctx)
		if err != nil {
			return SourceInfo{}, err
		}
		respFiles := make([]SourceInfoFile, 0, len(info.Files))
		for _, f := range info.Files {
			respFiles = append(respFiles, SourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
		}
		return SourceInfo{Files: respFiles, StorageTotal: info.StorageTotal, StorageUsed: info.StorageUsed, StorageFree: info.StorageFree}, nil
	default:
		return SourceInfo{}, ErrUnsupportedSourceType
	}
}

func CheckSourceHealth(ctx context.Context, src *repository.Source, factory SourceClientFactory) (SourceHealth, error) {
	if src == nil {
		return SourceHealth{}, errors.New("nil source")
	}
	start := time.Now()
	health := SourceHealth{
		SourceID:   src.ID.Hex(),
		SourceType: string(src.Type),
		Status:     SourceHealthHealthy,
		ReasonCode: "ok",
		Message:    "Source is reachable.",
	}
	finish := func(status SourceHealthStatus, reasonCode, message string) SourceHealth {
		health.Status = status
		health.ReasonCode = reasonCode
		health.Message = message
		health.CheckedAt = time.Now().UTC()
		health.LatencyMS = time.Since(start).Milliseconds()
		return health
	}

	cli, err := sourceClientFor(ctx, src, factory)
	if err != nil {
		if err == ErrUnsupportedSourceType {
			return SourceHealth{}, err
		}
		return finish(SourceHealthUnhealthy, "client_error", "Source configuration could not be initialized."), nil
	}

	switch src.Type {
	case repository.SourceTypeGDrive, repository.SourceTypeTelegram, repository.SourceTypeS3:
		healthClient, ok := cli.(sourcecap.HealthProber)
		if !ok {
			return SourceHealth{}, ErrUnsupportedSourceType
		}
		result, err := healthClient.ProbeSourceHealth(ctx)
		if errors.Is(err, sourcecap.ErrUnsupportedCapability) {
			return SourceHealth{}, ErrUnsupportedSourceType
		}
		if err != nil && result.ReasonCode == "" {
			result = sourcecap.Health{
				Status:     sourcecap.HealthUnhealthy,
				ReasonCode: "probe_failed",
				Message:    "Source health probe failed.",
			}
		}
		health.Quota = sourceQuotaFromCapability(result.Quota)
		return finish(sourceHealthStatusFromCapability(result.Status), result.ReasonCode, result.Message), nil
	default:
		return SourceHealth{}, ErrUnsupportedSourceType
	}
}

func sourceHealthStatusFromCapability(status sourcecap.HealthStatus) SourceHealthStatus {
	switch status {
	case sourcecap.HealthDegraded:
		return SourceHealthDegraded
	case sourcecap.HealthUnhealthy:
		return SourceHealthUnhealthy
	default:
		return SourceHealthHealthy
	}
}

func sourceQuotaFromCapability(quota sourcecap.Quota) SourceQuota {
	return SourceQuota{
		TotalBytes: quota.TotalBytes,
		UsedBytes:  quota.UsedBytes,
		FreeBytes:  quota.FreeBytes,
	}
}

func DownloadSourceFile(ctx context.Context, src *repository.Source, fileID string, factory SourceClientFactory) (io.ReadCloser, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive, repository.SourceTypeS3:
		cli, err := sourceClientFor(ctx, src, factory)
		if err != nil {
			return nil, err
		}
		if streamClient, ok := cli.(interface {
			DownloadStream(context.Context, string) (io.ReadCloser, error)
		}); ok {
			return streamClient.DownloadStream(ctx, fileID)
		}
		return cli.Download(ctx, fileID)
	default:
		return nil, ErrUnsupportedSourceType
	}
}

func sourceClientFor(ctx context.Context, src *repository.Source, factory SourceClientFactory) (SourceClient, error) {
	if factory == nil {
		factory = NewSourceClient
	}
	return factory(ctx, src)
}
