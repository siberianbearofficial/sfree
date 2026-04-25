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
	if info, ok := sourceTypeDefaultInfo(src.Type); ok {
		return info, nil
	}
	if !sourceTypeSupportsProviderInfo(src.Type) {
		return SourceInfo{}, ErrUnsupportedSourceType
	}
	cli, err := sourceClientFor(ctx, src, factory)
	if err != nil {
		return SourceInfo{}, err
	}
	return inspectSourceProvider(ctx, cli)
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

	if !sourceTypeSupportsHealth(src.Type) {
		return SourceHealth{}, ErrUnsupportedSourceType
	}
	result, err := probeSourceHealthProvider(ctx, cli)
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
	if !sourceTypeSupportsDirectDownload(src.Type) {
		return nil, ErrUnsupportedSourceType
	}
	cli, err := sourceClientFor(ctx, src, factory)
	if err != nil {
		return nil, err
	}
	return downloadSourceProviderFile(ctx, cli, fileID)
}

func sourceClientFor(ctx context.Context, src *repository.Source, factory SourceClientFactory) (SourceClient, error) {
	if factory == nil {
		factory = NewSourceClient
	}
	return factory(ctx, src)
}

func sourceTypeDefaultInfo(sourceType repository.SourceType) (SourceInfo, bool) {
	switch sourceType {
	case repository.SourceTypeTelegram:
		return SourceInfo{Files: []SourceInfoFile{}}, true
	default:
		return SourceInfo{}, false
	}
}

func sourceTypeSupportsProviderInfo(sourceType repository.SourceType) bool {
	switch sourceType {
	case repository.SourceTypeGDrive, repository.SourceTypeS3:
		return true
	default:
		return false
	}
}

func sourceTypeSupportsHealth(sourceType repository.SourceType) bool {
	switch sourceType {
	case repository.SourceTypeGDrive, repository.SourceTypeTelegram, repository.SourceTypeS3:
		return true
	default:
		return false
	}
}

func sourceTypeSupportsDirectDownload(sourceType repository.SourceType) bool {
	switch sourceType {
	case repository.SourceTypeGDrive, repository.SourceTypeS3:
		return true
	default:
		return false
	}
}
