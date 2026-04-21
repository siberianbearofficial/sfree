package manager

import (
	"context"
	"errors"
	"io"

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
