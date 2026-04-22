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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUnsupportedSourceType = errors.New("unsupported source type")

type SourceClient interface {
	Upload(ctx context.Context, name string, r io.Reader) (string, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

type sourceClient = SourceClient

type SourceClientFactory func(ctx context.Context, src *repository.Source) (SourceClient, error)

type sourceClientCache struct {
	factory SourceClientFactory
	clients map[primitive.ObjectID]sourceClient
}

func newSourceClientCache(factory SourceClientFactory) *sourceClientCache {
	if factory == nil {
		factory = NewSourceClient
	}
	return &sourceClientCache{
		factory: factory,
		clients: make(map[primitive.ObjectID]sourceClient),
	}
}

func (c *sourceClientCache) get(ctx context.Context, src repository.Source) (sourceClient, error) {
	cli, ok := c.clients[src.ID]
	if ok {
		return cli, nil
	}
	cli, err := c.factory(ctx, &src)
	if err != nil {
		return nil, err
	}
	c.clients[src.ID] = cli
	return cli, nil
}

func NewSourceClient(ctx context.Context, src *repository.Source) (SourceClient, error) {
	return NewSourceClientWithConfig(ctx, src, resilience.DefaultWrapperConfig())
}

func NewSourceClientWithConfig(ctx context.Context, src *repository.Source, resilienceConfig resilience.WrapperConfig) (SourceClient, error) {
	if src == nil {
		return nil, errors.New("nil source")
	}
	var (
		cli sourceClient
		err error
	)
	switch src.Type {
	case repository.SourceTypeGDrive:
		cli, err = gdrive.NewClient(ctx, []byte(src.Key))
	case repository.SourceTypeTelegram:
		var tcfg telegram.Config
		tcfg, err = telegram.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = telegram.NewClient(tcfg)
	case repository.SourceTypeS3:
		var scfg s3compat.Config
		scfg, err = s3compat.ParseConfig(src.Key)
		if err != nil {
			return nil, err
		}
		cli, err = s3compat.NewClient(ctx, scfg)
	default:
		return nil, ErrUnsupportedSourceType
	}
	if err != nil {
		return nil, err
	}
	return resilience.Wrap(cli, resilienceConfig), nil
}

func NewSourceClientFactory(resilienceConfig resilience.WrapperConfig) SourceClientFactory {
	return func(ctx context.Context, src *repository.Source) (SourceClient, error) {
		return NewSourceClientWithConfig(ctx, src, resilienceConfig)
	}
}

func sourceClientFactoryFromRepository(srcRepo *repository.SourceRepository, factory SourceClientFactory) SourceClientFactory {
	if factory == nil {
		factory = NewSourceClient
	}
	return func(ctx context.Context, src *repository.Source) (sourceClient, error) {
		fullSrc, err := srcRepo.GetByID(ctx, src.ID)
		if err != nil {
			return nil, err
		}
		return factory(ctx, fullSrc)
	}
}
