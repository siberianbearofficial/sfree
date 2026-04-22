package app

import (
	"net/http"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/docs"
	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/example/sfree/api-go/internal/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func registerProbeRoutes(router *gin.Engine, m *db.Mongo, limits *ratelimit.Limiters) {
	router.GET("/readyz", publicHandlers(limits, handlers.Readyz)...)
	router.GET("/healthz", publicHandlers(limits, handlers.Healthz)...)
	router.GET("/publication/ready", publicHandlers(limits, handlers.PublicationReady)...)
	router.GET("/dbz", publicHandlers(limits, handlers.DBProbe(m))...)
}

func registerRESTRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies, limits *ratelimit.Limiters) {
	registerUserRoutes(router, deps, limits)
	registerAuthRoutes(router, cfg, deps, limits)
	registerBucketRoutes(router, cfg, deps, limits)
	registerSourceRoutes(router, deps, limits)
}

func registerUserRoutes(router *gin.Engine, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.auth == nil || deps.userRepo == nil {
		return
	}
	router.POST("/api/v1/users", publicHandlers(limits, handlers.CreateUser(deps.userRepo))...)
}

func registerAuthRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies, limits *ratelimit.Limiters) {
	if cfg == nil || deps.userRepo == nil {
		return
	}
	router.GET("/api/v1/auth/github", publicHandlers(limits, handlers.GitHubLogin(cfg))...)
	router.GET("/api/v1/auth/github/callback", publicHandlers(limits, handlers.GitHubCallback(cfg, deps.userRepo))...)
	if deps.auth == nil {
		return
	}
	router.GET("/api/v1/auth/me", protectedHandlers(limits, deps.auth, handlers.GetCurrentUser(deps.userRepo))...)
	router.POST("/api/v1/auth/token", protectedHandlers(limits, deps.auth, handlers.TokenLogin(cfg, deps.userRepo))...)
}

func registerBucketRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.auth == nil || deps.bucketRepo == nil {
		return
	}
	router.POST("/api/v1/buckets", protectedHandlers(limits, deps.auth, handlers.CreateBucket(deps.bucketRepo, deps.sourceRepo, routerAccessSecret(cfg)))...)
	router.GET("/api/v1/buckets", protectedHandlers(limits, deps.auth, handlers.ListBuckets(deps.bucketRepo, deps.grantRepo))...)
	router.DELETE("/api/v1/buckets/:id", protectedHandlers(limits, deps.auth, handlers.DeleteBucket(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, deps.grantRepo))...)
	router.PATCH("/api/v1/buckets/:id/distribution", protectedHandlers(limits, deps.auth, handlers.UpdateBucketDistribution(deps.bucketRepo, deps.grantRepo))...)

	registerBucketGrantRoutes(router, deps, limits)
	registerBucketFileRoutes(router, cfg, deps, limits)
}

func registerBucketGrantRoutes(router *gin.Engine, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.grantRepo == nil || deps.userRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/grants", protectedHandlers(limits, deps.auth, handlers.CreateGrant(deps.bucketRepo, deps.grantRepo, deps.userRepo))...)
	router.GET("/api/v1/buckets/:id/grants", protectedHandlers(limits, deps.auth, handlers.ListGrants(deps.bucketRepo, deps.grantRepo, deps.userRepo))...)
	router.PATCH("/api/v1/buckets/:id/grants/:grant_id", protectedHandlers(limits, deps.auth, handlers.UpdateGrant(deps.bucketRepo, deps.grantRepo))...)
	router.DELETE("/api/v1/buckets/:id/grants/:grant_id", protectedHandlers(limits, deps.auth, handlers.DeleteGrant(deps.bucketRepo, deps.grantRepo))...)
}

func registerBucketFileRoutes(router *gin.Engine, cfg *config.Config, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/upload", protectedHandlers(limits, deps.auth, handlers.UploadFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo, routerUploadChunkSize(cfg)))...)
	router.GET("/api/v1/buckets/:id/files", protectedHandlers(limits, deps.auth, handlers.ListFiles(deps.bucketRepo, deps.fileRepo, deps.grantRepo))...)
	router.GET("/api/v1/buckets/:id/files/:file_id/download", protectedHandlers(limits, deps.auth, handlers.DownloadFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo))...)
	router.DELETE("/api/v1/buckets/:id/files/:file_id", protectedHandlers(limits, deps.auth, handlers.DeleteFile(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo))...)
	if deps.shareLinkRepo == nil {
		return
	}
	router.POST("/api/v1/buckets/:id/files/:file_id/share", protectedHandlers(limits, deps.auth, handlers.CreateShareLink(deps.bucketRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo))...)
	router.GET("/api/v1/buckets/:id/files/:file_id/shares", protectedHandlers(limits, deps.auth, handlers.ListShareLinks(deps.bucketRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo))...)
	router.DELETE("/api/v1/shares/:id", protectedHandlers(limits, deps.auth, handlers.DeleteShareLink(deps.shareLinkRepo))...)
}

func registerSourceRoutes(router *gin.Engine, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.auth == nil || deps.sourceRepo == nil {
		return
	}
	router.POST("/api/v1/sources/gdrive", protectedHandlers(limits, deps.auth, handlers.CreateGDriveSource(deps.sourceRepo))...)
	router.POST("/api/v1/sources/telegram", protectedHandlers(limits, deps.auth, handlers.CreateTelegramSource(deps.sourceRepo))...)
	router.POST("/api/v1/sources/s3", protectedHandlers(limits, deps.auth, handlers.CreateS3Source(deps.sourceRepo))...)
	router.GET("/api/v1/sources", protectedHandlers(limits, deps.auth, handlers.ListSources(deps.sourceRepo))...)
	router.GET("/api/v1/sources/:id/health", protectedHandlers(limits, deps.auth, handlers.GetSourceHealth(deps.sourceRepo))...)
	router.GET("/api/v1/sources/:id/info", protectedHandlers(limits, deps.auth, handlers.GetSourceInfo(deps.sourceRepo))...)
	router.GET("/api/v1/sources/:id/files/:file_id/download", protectedHandlers(limits, deps.auth, handlers.DownloadSourceFile(deps.sourceRepo))...)
	router.DELETE("/api/v1/sources/:id", protectedHandlers(limits, deps.auth, handlers.DeleteSource(deps.sourceRepo, deps.bucketRepo))...)
}

func registerPublicShareRoutes(router *gin.Engine, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.shareLinkRepo == nil || deps.bucketRepo == nil || deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	router.GET("/share/:token", publicHandlers(limits, handlers.GetSharedFile(deps.shareLinkRepo, deps.bucketRepo, deps.sourceRepo, deps.fileRepo))...)
}

func registerDocsMetricsRoutes(router *gin.Engine, limits *ratelimit.Limiters) {
	router.GET("/api/openapi.json", publicHandlers(limits, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", docs.OpenAPIJSON())
	})...)
	router.GET("/api/docs", publicHandlers(limits, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/docs/index.html")
	})...)
	router.GET("/api/docs/*any", publicHandlers(limits, ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/openapi.json")))...)
	router.GET("/swagger/*any", publicHandlers(limits, ginSwagger.WrapHandler(swaggerFiles.Handler))...)
	router.GET("/metrics", publicHandlers(limits, gin.WrapH(promhttp.Handler()))...)
}

func registerS3Routes(router *gin.Engine, cfg *config.Config, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.bucketRepo == nil || deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	s3Auth := handlers.AWS4Auth(deps.bucketRepo, routerAccessSecret(cfg))
	router.HEAD("/api/s3/:bucket/*object", protectedHandlers(limits, s3Auth, handlers.HeadObject(deps.bucketRepo, deps.fileRepo))...)
	router.GET("/api/s3/:bucket/*object", protectedHandlers(limits, s3Auth, handlers.GetObjectOrParts(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo))...)
	router.GET("/api/s3/:bucket", protectedHandlers(limits, s3Auth, handlers.ListObjectsOrUploads(deps.bucketRepo, deps.fileRepo, deps.mpRepo))...)
	router.PUT("/api/s3/:bucket/*object", protectedHandlers(limits, s3Auth, handlers.PutObjectOrPart(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, routerUploadChunkSize(cfg)))...)
	router.POST("/api/s3/:bucket", protectedHandlers(limits, s3Auth, handlers.PostBucket(deps.bucketRepo, deps.sourceRepo, deps.fileRepo))...)
	router.POST("/api/s3/:bucket/*object", protectedHandlers(limits, s3Auth, handlers.PostObject(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, routerUploadChunkSize(cfg)))...)
	router.DELETE("/api/s3/:bucket/*object", protectedHandlers(limits, s3Auth, handlers.DeleteObjectOrAbort(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo))...)
}

func publicHandlers(limits *ratelimit.Limiters, handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	return append([]gin.HandlerFunc{limits.IPMiddleware()}, handlers...)
}

func protectedHandlers(limits *ratelimit.Limiters, auth gin.HandlerFunc, handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	chain := []gin.HandlerFunc{limits.PreAuthIPMiddleware(), auth, limits.IdentityMiddleware()}
	return append(chain, handlers...)
}
