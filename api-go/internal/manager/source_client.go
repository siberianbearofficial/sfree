package manager

import (
	"context"
	"errors"
	"io"

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/resilience"
	"github.com/example/sfree/api-go/internal/s3compat"
	"github.com/example/sfree/api-go/internal/telegram"
)

var ErrSourceDownloadUnsupported = errors.New("source download unsupported")

type SourceInfoClient interface {
	Info(ctx context.Context) (SourceInfo, error)
}

type DirectSourceClient interface {
	Download(ctx context.Context, name string) (io.ReadCloser, error)
}

type fullGDriveSourceClient interface {
	sourceClient
	ListFiles(ctx context.Context) ([]gdrive.File, error)
	StorageInfo(ctx context.Context) (total, used, free int64, err error)
}

type fullS3SourceClient interface {
	sourceClient
	ListObjects(ctx context.Context) ([]s3compat.ObjectInfo, int64, error)
}

type sourceClientBuilder struct {
	newGDriveClient   func(ctx context.Context, credsJSON []byte) (fullGDriveSourceClient, error)
	newTelegramClient func(cfg telegram.Config) (sourceClient, error)
	newS3Client       func(ctx context.Context, cfg s3compat.Config) (fullS3SourceClient, error)
}

func NewSourceInfoClient(ctx context.Context, src *repository.Source) (SourceInfoClient, error) {
	return sourceClientBuilder{}.NewSourceInfoClient(ctx, src)
}

func NewDirectSourceClient(ctx context.Context, src *repository.Source) (DirectSourceClient, error) {
	return sourceClientBuilder{}.NewDirectSourceClient(ctx, src)
}

func NewSourceClient(ctx context.Context, src *repository.Source) (sourceClient, error) {
	return sourceClientBuilder{}.NewStorageSourceClient(ctx, src)
}

func (b sourceClientBuilder) NewSourceInfoClient(ctx context.Context, src *repository.Source) (SourceInfoClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive:
		cli, err := b.gdriveClient(ctx, []byte(src.Key))
		if err != nil {
			return nil, err
		}
		return gdriveSourceInfoClient{client: cli}, nil
	case repository.SourceTypeTelegram:
		return emptySourceInfoClient{}, nil
	case repository.SourceTypeS3:
		cfg, err := s3compat.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err := b.s3Client(ctx, cfg)
		if err != nil {
			return nil, err
		}
		return s3SourceInfoClient{client: cli}, nil
	default:
		return nil, ErrUnsupportedSourceType
	}
}

func (b sourceClientBuilder) NewDirectSourceClient(ctx context.Context, src *repository.Source) (DirectSourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	switch src.Type {
	case repository.SourceTypeGDrive, repository.SourceTypeS3:
		cli, err := b.NewStorageSourceClient(ctx, src)
		if err != nil {
			return nil, err
		}
		return directSourceClient{client: cli}, nil
	default:
		return nil, ErrSourceDownloadUnsupported
	}
}

func (b sourceClientBuilder) NewStorageSourceClient(ctx context.Context, src *repository.Source) (sourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	var (
		cli sourceClient
		err error
	)
	switch src.Type {
	case repository.SourceTypeGDrive:
		cli, err = b.gdriveClient(ctx, []byte(src.Key))
	case repository.SourceTypeTelegram:
		var cfg telegram.Config
		cfg, err = telegram.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = b.telegramClient(cfg)
	case repository.SourceTypeS3:
		var cfg s3compat.Config
		cfg, err = s3compat.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = b.s3Client(ctx, cfg)
	default:
		return nil, ErrUnsupportedSourceType
	}
	if err != nil {
		return nil, err
	}
	return resilience.Wrap(cli, ResilienceConfig), nil
}

func (b sourceClientBuilder) gdriveClient(ctx context.Context, credsJSON []byte) (fullGDriveSourceClient, error) {
	if b.newGDriveClient != nil {
		return b.newGDriveClient(ctx, credsJSON)
	}
	return gdrive.NewClient(ctx, credsJSON)
}

func (b sourceClientBuilder) telegramClient(cfg telegram.Config) (sourceClient, error) {
	if b.newTelegramClient != nil {
		return b.newTelegramClient(cfg)
	}
	return telegram.NewClient(cfg)
}

func (b sourceClientBuilder) s3Client(ctx context.Context, cfg s3compat.Config) (fullS3SourceClient, error) {
	if b.newS3Client != nil {
		return b.newS3Client(ctx, cfg)
	}
	return s3compat.NewClient(ctx, cfg)
}

type gdriveSourceInfoClient struct {
	client fullGDriveSourceClient
}

func (c gdriveSourceInfoClient) Info(ctx context.Context) (SourceInfo, error) {
	files, err := c.client.ListFiles(ctx)
	if err != nil {
		return SourceInfo{}, err
	}
	total, used, free, err := c.client.StorageInfo(ctx)
	if err != nil {
		return SourceInfo{}, err
	}
	respFiles := make([]SourceInfoFile, 0, len(files))
	for _, f := range files {
		respFiles = append(respFiles, SourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
	}
	return SourceInfo{
		Files:        respFiles,
		StorageTotal: total,
		StorageUsed:  used,
		StorageFree:  free,
	}, nil
}

type s3SourceInfoClient struct {
	client fullS3SourceClient
}

func (c s3SourceInfoClient) Info(ctx context.Context) (SourceInfo, error) {
	objects, used, err := c.client.ListObjects(ctx)
	if err != nil {
		return SourceInfo{}, err
	}
	respFiles := make([]SourceInfoFile, 0, len(objects))
	for _, obj := range objects {
		respFiles = append(respFiles, SourceInfoFile{ID: obj.Key, Name: obj.Key, Size: obj.Size})
	}
	return SourceInfo{
		Files:       respFiles,
		StorageUsed: used,
	}, nil
}

type emptySourceInfoClient struct{}

func (emptySourceInfoClient) Info(context.Context) (SourceInfo, error) {
	return SourceInfo{Files: []SourceInfoFile{}}, nil
}

type directSourceClient struct {
	client sourceClient
}

func (c directSourceClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	if streamClient, ok := c.client.(interface {
		DownloadStream(context.Context, string) (io.ReadCloser, error)
	}); ok {
		return streamClient.DownloadStream(ctx, name)
	}
	return c.client.Download(ctx, name)
}
