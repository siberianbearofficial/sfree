package app

import (
	"fmt"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/resilience"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type routerDependencies struct {
	auth          gin.HandlerFunc
	userRepo      *repository.UserRepository
	bucketRepo    *repository.BucketRepository
	sourceRepo    *repository.SourceRepository
	fileRepo      *repository.FileRepository
	shareLinkRepo *repository.ShareLinkRepository
	mpRepo        *repository.MultipartUploadRepository
	grantRepo     *repository.BucketGrantRepository
	sourceFactory manager.SourceClientFactory
}

type routerDependencyConstructors struct {
	user            func(*mongo.Database) (*repository.UserRepository, error)
	bucket          func(*mongo.Database) (*repository.BucketRepository, error)
	source          func(*mongo.Database, ...string) (*repository.SourceRepository, error)
	file            func(*mongo.Database) (*repository.FileRepository, error)
	shareLink       func(*mongo.Database) (*repository.ShareLinkRepository, error)
	multipartUpload func(*mongo.Database) (*repository.MultipartUploadRepository, error)
	bucketGrant     func(*mongo.Database) (*repository.BucketGrantRepository, error)
}

func (constructors routerDependencyConstructors) withDefaults() routerDependencyConstructors {
	if constructors.user == nil {
		constructors.user = repository.NewUserRepository
	}
	if constructors.bucket == nil {
		constructors.bucket = repository.NewBucketRepository
	}
	if constructors.source == nil {
		constructors.source = repository.NewSourceRepository
	}
	if constructors.file == nil {
		constructors.file = repository.NewFileRepository
	}
	if constructors.shareLink == nil {
		constructors.shareLink = repository.NewShareLinkRepository
	}
	if constructors.multipartUpload == nil {
		constructors.multipartUpload = repository.NewMultipartUploadRepository
	}
	if constructors.bucketGrant == nil {
		constructors.bucketGrant = repository.NewBucketGrantRepository
	}
	return constructors
}

func newRouterDependencies(m *db.Mongo, cfg *config.Config, constructors routerDependencyConstructors) (*routerDependencies, error) {
	deps := &routerDependencies{sourceFactory: routerSourceClientFactory(cfg)}
	if m == nil {
		return deps, nil
	}
	constructors = constructors.withDefaults()

	var err error
	deps.userRepo, err = constructors.user(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize user repository: %w", err)
	}
	deps.auth = handlers.Auth(deps.userRepo, routerJWTSecret(cfg))

	deps.bucketRepo, err = constructors.bucket(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize bucket repository: %w", err)
	}
	deps.sourceRepo, err = constructors.source(m.DB, routerAccessSecret(cfg))
	if err != nil {
		return nil, fmt.Errorf("initialize source repository: %w", err)
	}
	deps.fileRepo, err = constructors.file(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize file repository: %w", err)
	}
	deps.mpRepo, err = constructors.multipartUpload(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize multipart upload repository: %w", err)
	}
	deps.shareLinkRepo, err = constructors.shareLink(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize share link repository: %w", err)
	}
	deps.grantRepo, err = constructors.bucketGrant(m.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize bucket grant repository: %w", err)
	}

	return deps, nil
}

func routerJWTSecret(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.JWTSecret != "" {
		return cfg.JWTSecret
	}
	return cfg.AccessSecretKey
}

func routerAccessSecret(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.AccessSecretKey
}

func routerUploadChunkSize(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.Upload.ChunkSize
}

func routerSourceClientFactory(cfg *config.Config) manager.SourceClientFactory {
	return manager.NewSourceClientFactory(routerSourceClientResilienceConfig(cfg))
}

func routerSourceClientResilienceConfig(cfg *config.Config) resilience.WrapperConfig {
	rcfg := resilience.DefaultWrapperConfig()
	if cfg == nil {
		return rcfg
	}
	if cfg.SourceClient.TimeoutSeconds > 0 {
		rcfg.Timeout = time.Duration(cfg.SourceClient.TimeoutSeconds) * time.Second
	}
	if cfg.SourceClient.FailureThreshold > 0 {
		rcfg.FailureThreshold = cfg.SourceClient.FailureThreshold
	}
	if cfg.SourceClient.RecoverySeconds > 0 {
		rcfg.RecoveryTimeout = time.Duration(cfg.SourceClient.RecoverySeconds) * time.Second
	}
	if cfg.SourceClient.MaxRetries > 0 {
		rcfg.MaxRetries = cfg.SourceClient.MaxRetries
	}
	if cfg.SourceClient.RetryBaseDelayMs > 0 {
		rcfg.RetryBaseDelay = time.Duration(cfg.SourceClient.RetryBaseDelayMs) * time.Millisecond
	}
	if cfg.SourceClient.RetryMaxDelayMs > 0 {
		rcfg.RetryMaxDelay = time.Duration(cfg.SourceClient.RetryMaxDelayMs) * time.Millisecond
	}
	return rcfg
}
