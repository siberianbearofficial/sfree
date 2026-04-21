package app

import (
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
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
}

func newRouterDependencies(m *db.Mongo, cfg *config.Config) *routerDependencies {
	deps := &routerDependencies{}
	if m == nil {
		return deps
	}

	var err error
	deps.userRepo, err = repository.NewUserRepository(m.DB)
	if err == nil {
		deps.auth = handlers.Auth(deps.userRepo, routerJWTSecret(cfg))
	}
	deps.bucketRepo, _ = repository.NewBucketRepository(m.DB)
	deps.sourceRepo, _ = repository.NewSourceRepository(m.DB, routerAccessSecret(cfg))
	deps.fileRepo, _ = repository.NewFileRepository(m.DB)
	deps.mpRepo, _ = repository.NewMultipartUploadRepository(m.DB)
	deps.shareLinkRepo, _ = repository.NewShareLinkRepository(m.DB)
	deps.grantRepo, _ = repository.NewBucketGrantRepository(m.DB)

	return deps
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
