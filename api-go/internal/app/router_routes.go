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
	router.GET("/api/v1/buckets/:id", protectedHandlers(limits, deps.auth, handlers.GetBucket(deps.bucketRepo, deps.grantRepo))...)
	router.DELETE("/api/v1/buckets/:id", protectedHandlers(limits, deps.auth, handlers.DeleteBucketWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, deps.shareLinkRepo, deps.grantRepo, deps.sourceFactory))...)
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
	router.POST("/api/v1/buckets/:id/upload", protectedHandlers(limits, deps.auth, handlers.UploadFileWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo, routerUploadChunkSize(cfg), deps.sourceFactory))...)
	router.GET("/api/v1/buckets/:id/files", protectedHandlers(limits, deps.auth, handlers.ListFiles(deps.bucketRepo, deps.fileRepo, deps.grantRepo))...)
	router.POST("/api/v1/buckets/:id/files/batch-delete", protectedHandlers(limits, deps.auth, handlers.BatchDeleteFilesWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo, deps.sourceFactory))...)
	router.GET("/api/v1/buckets/:id/files/:file_id/download", protectedHandlers(limits, deps.auth, handlers.DownloadFileWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.grantRepo, deps.sourceFactory))...)
	router.DELETE("/api/v1/buckets/:id/files/:file_id", protectedHandlers(limits, deps.auth, handlers.DeleteFileWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.shareLinkRepo, deps.grantRepo, deps.sourceFactory))...)
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
	router.GET("/api/v1/sources/:id/health", protectedHandlers(limits, deps.auth, handlers.GetSourceHealthWithFactory(deps.sourceRepo, deps.sourceFactory))...)
	router.GET("/api/v1/sources/:id/info", protectedHandlers(limits, deps.auth, handlers.GetSourceInfoWithFactory(deps.sourceRepo, deps.sourceFactory))...)
	router.GET("/api/v1/sources/:id/download", protectedHandlers(limits, deps.auth, handlers.DownloadSourceFileByQueryWithFactory(deps.sourceRepo, deps.sourceFactory))...)
	router.GET("/api/v1/sources/:id/files/:file_id/download", protectedHandlers(limits, deps.auth, handlers.DownloadSourceFileWithFactory(deps.sourceRepo, deps.sourceFactory))...)
	router.DELETE("/api/v1/sources/:id", protectedHandlers(limits, deps.auth, handlers.DeleteSource(deps.sourceRepo, deps.bucketRepo))...)
}

func registerPublicShareRoutes(router *gin.Engine, deps *routerDependencies, limits *ratelimit.Limiters) {
	if deps.shareLinkRepo == nil || deps.bucketRepo == nil || deps.sourceRepo == nil || deps.fileRepo == nil {
		return
	}
	router.GET("/share/:token", publicHandlers(limits, handlers.GetSharedFileWithFactory(deps.shareLinkRepo, deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.sourceFactory))...)
}

func registerDocsMetricsRoutes(router *gin.Engine, limits *ratelimit.Limiters) {
	router.GET("/api/openapi.json", publicHandlers(limits, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
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
	uploadChunkSize := routerUploadChunkSize(cfg)
	listBuckets := handlers.ListBucketsS3(deps.bucketRepo)
	headBucket := handlers.HeadBucket(deps.bucketRepo)
	headObject := handlers.HeadObject(deps.bucketRepo, deps.fileRepo)
	getObjectOrParts := handlers.GetObjectOrPartsWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, deps.sourceFactory)
	listObjectsOrUploads := handlers.ListObjectsOrUploads(deps.bucketRepo, deps.fileRepo, deps.mpRepo)
	putObjectOrPart := handlers.PutObjectOrPartWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, uploadChunkSize, deps.sourceFactory)
	postBucket := handlers.PostBucketWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.sourceFactory)
	postObject := handlers.PostObjectWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, uploadChunkSize, deps.sourceFactory)
	deleteObjectOrAbort := handlers.DeleteObjectOrAbortWithFactory(deps.bucketRepo, deps.sourceRepo, deps.fileRepo, deps.mpRepo, deps.sourceFactory)

	getDispatch := s3GetDispatchHandler(listBuckets, listObjectsOrUploads, getObjectOrParts)
	headDispatch := s3HeadDispatchHandler(headBucket, headObject)
	putDispatch := s3PutDispatchHandler(putObjectOrPart)
	postDispatch := s3PostDispatchHandler(postBucket, postObject)
	deleteDispatch := s3DeleteDispatchHandler(deleteObjectOrAbort)

	for _, root := range []string{"/", "/api/s3"} {
		router.GET(root, protectedHandlers(limits, s3Auth, getDispatch)...)
		if root == "/" {
			router.HEAD(root, protectedHandlers(limits, s3Auth, headDispatch)...)
			router.POST(root, protectedHandlers(limits, s3Auth, postDispatch)...)
		}
	}
	for _, prefix := range []string{"", "/api/s3"} {
		bucketPath, objectPath := s3RoutePaths(prefix)
		router.HEAD(bucketPath, protectedHandlers(limits, s3Auth, headDispatch)...)
		router.HEAD(objectPath, protectedHandlers(limits, s3Auth, headDispatch)...)
		router.GET(objectPath, protectedHandlers(limits, s3Auth, getDispatch)...)
		router.GET(bucketPath, protectedHandlers(limits, s3Auth, getDispatch)...)
		router.PUT(objectPath, protectedHandlers(limits, s3Auth, putDispatch)...)
		router.POST(bucketPath, protectedHandlers(limits, s3Auth, postDispatch)...)
		router.POST(objectPath, protectedHandlers(limits, s3Auth, postDispatch)...)
		router.DELETE(objectPath, protectedHandlers(limits, s3Auth, deleteDispatch)...)
		if prefix == "" {
			router.PUT(bucketPath, protectedHandlers(limits, s3Auth, putDispatch)...)
			router.DELETE(bucketPath, protectedHandlers(limits, s3Auth, deleteDispatch)...)
		}
	}
}

func s3RoutePaths(prefix string) (bucketPath, objectPath string) {
	bucketPath = prefix + "/:bucket"
	objectPath = prefix + "/:bucket/*object"
	return bucketPath, objectPath
}

func publicHandlers(limits *ratelimit.Limiters, handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	return append([]gin.HandlerFunc{limits.IPMiddleware()}, handlers...)
}

func protectedHandlers(limits *ratelimit.Limiters, auth gin.HandlerFunc, handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	chain := []gin.HandlerFunc{limits.PreAuthIPMiddleware(), auth, limits.IdentityMiddleware()}
	return append(chain, handlers...)
}
