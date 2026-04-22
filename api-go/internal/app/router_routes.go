package app

import (
	"net/http"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/docs"
	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func registerProbeRoutes(router *gin.Engine, m *db.Mongo) {
	router.GET("/readyz", handlers.Readyz)
	router.GET("/healthz", handlers.Healthz)
	router.GET("/publication/ready", handlers.PublicationReady)
	router.GET("/dbz", handlers.DBProbe(m))
}

func registerRESTRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies) {
	registerUserRoutes(router, deps)
	registerAuthRoutes(router, cfg, deps)
	registerBucketRoutes(router, cfg, deps)
	registerSourceRoutes(router, deps)
}

func registerUserRoutes(router *gin.Engine, deps *routerDependencies) {
	if deps.auth == nil || deps.userRepo == nil {
		return
	}
	router.POST("/api/v1/users", handlers.CreateUser(deps.userRepo))
}

func registerAuthRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies) {
	if cfg == nil || deps.userRepo == nil {
		return
	}
	router.GET("/api/v1/auth/github", handlers.GitHubLogin(cfg))
	router.GET("/api/v1/auth/github/callback", handlers.GitHubCallback(cfg, deps.userRepo))
	if deps.auth == nil {
		return
	}
	router.GET("/api/v1/auth/me", deps.auth, handlers.GetCurrentUser(deps.userRepo))
	router.POST("/api/v1/auth/token", deps.auth, handlers.TokenLogin(cfg, deps.userRepo))
}

func registerBucketRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies) {
	if deps.auth == nil || deps.bucketRepo == nil {
		return
	}
	router.POST("/api/v1/buckets", deps.auth, handlers.CreateBucket(deps.bucketRepo, deps.sourceRepo, routerAccessSecret(cfg)))
	router.GET("/api/v1/buckets", deps.auth, handlers.ListBuckets(deps.bucketRepo, deps.grantRepo))
	router.DELETE("/api/v1/buckets/:id", deps.auth, handlers.DeleteBucket(deps.bucketRepo, deps.grantRepo))
	router.PATCH("/api/v1/buckets/:id/distribution", deps.auth, handlers.UpdateBucketDistribution(deps.bucketRepo, deps.grantRepo))

	registerBucketGrantRoutes(router, deps)
	registerBucketFileRoutes(router, cfg, deps)
}

func registerBucketGrantRoutes(router *gin.Engine, deps *routerDependencies) {
	if deps.grantRepo == nil || deps.userRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/grants", deps.auth, handlers.CreateGrant(deps.bucketRepo, deps.grantRepo, deps.userRepo))
	router.GET("/api/v1/buckets/:id/grants", deps.auth, handlers.ListGrants(deps.bucketRepo, deps.grantRepo, deps.userRepo))
	router.PATCH("/api/v1/buckets/:id/grants/:grant_id", deps.auth, handlers.UpdateGrant(deps.bucketRepo, deps.grantRepo))
	router.DELETE("/api/v1/buckets/:id/grants/:grant_id", deps.auth, handlers.DeleteGrant(deps.bucketRepo, deps.grantRepo))
}

func registerBucketFileRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies) {
	if deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/upload", deps.auth, handlers.UploadFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo, routerUploadChunkSize(cfg)))
	router.GET("/api/v1/buckets/:id/files", deps.auth, handlers.ListFiles(deps.bucketRepo, deps.fileRepo, deps.grantRepo))
	router.GET("/api/v1/buckets/:id/files/:file_id/download", deps.auth, handlers.DownloadFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo))
	router.DELETE("/api/v1/buckets/:id/files/:file_id", deps.auth, handlers.DeleteFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo))
	if deps.shareLinkRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/files/:file_id/share", deps.auth, handlers.CreateShareLink(deps.bucketRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo))
	router.GET("/api/v1/buckets/:id/files/:file_id/shares", deps.auth, handlers.ListShareLinks(deps.bucketRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo))
	router.DELETE("/api/v1/shares/:id", deps.auth, handlers.DeleteShareLink(deps.shareLinkRepo))
}

func registerSourceRoutes(router *gin.Engine, deps *routerDependencies) {
	if deps.auth == nil || deps.sourceRepo == nil {
		return
	}
	router.POST("/api/v1/sources/gdrive", deps.auth, handlers.CreateGDriveSource(deps.sourceRepo))
	router.POST("/api/v1/sources/telegram", deps.auth, handlers.CreateTelegramSource(deps.sourceRepo))
	router.POST("/api/v1/sources/s3", deps.auth, handlers.CreateS3Source(deps.sourceRepo))
	router.GET("/api/v1/sources", deps.auth, handlers.ListSources(deps.sourceRepo))
	router.GET("/api/v1/sources/:id/info", deps.auth, handlers.GetSourceInfo(deps.sourceRepo))
	router.GET("/api/v1/sources/:id/files/:file_id/download", deps.auth, handlers.DownloadSourceFile(deps.sourceRepo))
	router.DELETE("/api/v1/sources/:id", deps.auth, handlers.DeleteSource(deps.sourceRepo, deps.bucketRepo))
}

func registerPublicShareRoutes(router *gin.Engine, deps *routerDependencies) {
	if deps.shareLinkRepo == nil || deps.bucketRepo == nil || deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	router.GET("/share/:token", handlers.GetSharedFile(deps.shareLinkRepo, deps.bucketRepo, deps.sourceRepo, deps.fileRepo))
}

func registerDocsMetricsRoutes(router *gin.Engine) {
	router.GET("/api/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", docs.OpenAPIJSON())
	})
	router.GET("/api/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/docs/index.html")
	})
	router.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/openapi.json")))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

func registerS3Routes(router *gin.Engine, cfg *config.Config, deps *routerDependencies) {
	if deps.bucketRepo == nil || deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	s3Auth := handlers.AWS4Auth(deps.bucketRepo, routerAccessSecret(cfg))
	router.HEAD("/api/s3/:bucket/*object", s3Auth, handlers.HeadObject(deps.bucketRepo, deps.fileRepo))
	router.GET("/api/s3/:bucket/*object", s3Auth, handlers.GetObjectOrParts(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo))
	router.GET("/api/s3/:bucket", s3Auth, handlers.ListObjectsOrUploads(deps.bucketRepo, deps.fileRepo, deps.mpRepo))
	router.PUT("/api/s3/:bucket/*object", s3Auth, handlers.PutObjectOrPart(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, routerUploadChunkSize(cfg)))
	router.POST("/api/s3/:bucket", s3Auth, handlers.PostBucket(deps.bucketRepo, deps.sourceRepo, deps.fileRepo))
	router.POST("/api/s3/:bucket/*object", s3Auth, handlers.PostObject(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, routerUploadChunkSize(cfg)))
	router.DELETE("/api/s3/:bucket/*object", s3Auth, handlers.DeleteObjectOrAbort(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo))
}
