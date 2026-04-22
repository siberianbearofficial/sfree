package manager

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3compat"
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
		infoClient, ok := cli.(interface {
			ListFiles(context.Context) ([]gdrive.File, error)
			StorageInfo(context.Context) (int64, int64, int64, error)
		})
		if !ok {
			return SourceInfo{}, ErrUnsupportedSourceType
		}
		files, err := infoClient.ListFiles(ctx)
		if err != nil {
			return SourceInfo{}, err
		}
		total, used, free, err := infoClient.StorageInfo(ctx)
		if err != nil {
			return SourceInfo{}, err
		}
		respFiles := make([]SourceInfoFile, 0, len(files))
		for _, f := range files {
			respFiles = append(respFiles, SourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
		}
		return SourceInfo{Files: respFiles, StorageTotal: total, StorageUsed: used, StorageFree: free}, nil
	case repository.SourceTypeTelegram:
		return SourceInfo{Files: []SourceInfoFile{}}, nil
	case repository.SourceTypeS3:
		cli, err := sourceClientFor(ctx, src, factory)
		if err != nil {
			return SourceInfo{}, err
		}
		infoClient, ok := cli.(interface {
			ListObjects(context.Context) ([]s3compat.ObjectInfo, int64, error)
		})
		if !ok {
			return SourceInfo{}, ErrUnsupportedSourceType
		}
		files, used, err := infoClient.ListObjects(ctx)
		if err != nil {
			return SourceInfo{}, err
		}
		respFiles := make([]SourceInfoFile, 0, len(files))
		for _, f := range files {
			respFiles = append(respFiles, SourceInfoFile{ID: f.Key, Name: f.Key, Size: f.Size})
		}
		return SourceInfo{Files: respFiles, StorageUsed: used}, nil
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
	case repository.SourceTypeGDrive:
		infoClient, ok := cli.(interface {
			StorageInfo(context.Context) (int64, int64, int64, error)
		})
		if !ok {
			return SourceHealth{}, ErrUnsupportedSourceType
		}
		total, used, free, err := infoClient.StorageInfo(ctx)
		if err != nil {
			return finish(SourceHealthUnhealthy, "probe_failed", "Google Drive metadata probe failed."), nil
		}
		if total > 0 {
			health.Quota = SourceQuota{TotalBytes: &total, UsedBytes: &used, FreeBytes: &free}
			if free <= 0 {
				return finish(SourceHealthUnhealthy, "quota_exhausted", "Google Drive quota is exhausted."), nil
			}
			if free*100/total < 5 {
				return finish(SourceHealthDegraded, "quota_low", "Google Drive quota is nearly exhausted."), nil
			}
		}
		return finish(SourceHealthHealthy, "ok", "Google Drive metadata is reachable."), nil
	case repository.SourceTypeTelegram:
		healthClient, ok := cli.(interface {
			CheckChat(context.Context) error
		})
		if !ok {
			return SourceHealth{}, ErrUnsupportedSourceType
		}
		if err := healthClient.CheckChat(ctx); err != nil {
			return finish(SourceHealthUnhealthy, "probe_failed", "Telegram bot or chat is not reachable."), nil
		}
		return finish(SourceHealthHealthy, "ok", "Telegram bot and chat are reachable."), nil
	case repository.SourceTypeS3:
		healthClient, ok := cli.(interface {
			HeadBucket(context.Context) error
		})
		if !ok {
			return SourceHealth{}, ErrUnsupportedSourceType
		}
		if err := healthClient.HeadBucket(ctx); err != nil {
			return finish(SourceHealthUnhealthy, "probe_failed", "S3 bucket metadata probe failed."), nil
		}
		return finish(SourceHealthHealthy, "ok", "S3 bucket metadata is reachable."), nil
	default:
		return SourceHealth{}, ErrUnsupportedSourceType
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
