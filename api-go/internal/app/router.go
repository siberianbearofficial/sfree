package app

import (
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/example/sfree/api-go/internal/observability"
	"github.com/example/sfree/api-go/internal/ratelimit"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func SetupRouter(m *db.Mongo, cfg *config.Config) *gin.Engine {
	router := gin.New()

	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}
	if cfg != nil && cfg.FrontendURL != "" {
		corsConfig.AllowOrigins = []string{cfg.FrontendURL}
	} else {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowCredentials = false
	}
	router.Use(cors.New(corsConfig))
	router.Use(gin.Recovery())
	router.Use(observability.Middleware())
	router.Use(otelgin.Middleware("sfree-api"))

	rlCfg := ratelimit.DefaultConfig()
	if cfg != nil {
		if cfg.RateLimit.PerIP > 0 {
			rlCfg.PerIPReqsPerMin = cfg.RateLimit.PerIP
		}
		if cfg.RateLimit.PerKey > 0 {
			rlCfg.PerKeyReqsPerMin = cfg.RateLimit.PerKey
		}
	}
	router.Use(ratelimit.Middleware(rlCfg))

	router.GET("/readyz", handlers.Readyz)
	router.GET("/healthz", handlers.Healthz)
	router.GET("/publication/ready", handlers.PublicationReady)
	router.GET("/dbz", handlers.DBProbe(m))

	var (
		auth          gin.HandlerFunc
		userRepo      *repository.UserRepository
		bucketRepo    *repository.BucketRepository
		sourceRepo    *repository.SourceRepository
		fileRepo      *repository.FileRepository
		shareLinkRepo *repository.ShareLinkRepository
		mpRepo        *repository.MultipartUploadRepository
	)

	jwtSecret := ""
	if cfg != nil {
		jwtSecret = cfg.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.AccessSecretKey
		}
	}

	if m != nil {
		var err error
		userRepo, err = repository.NewUserRepository(m.DB)
		if err == nil {
			auth = handlers.Auth(userRepo, jwtSecret)
			router.POST("/api/v1/users", handlers.CreateUser(userRepo))
		}
		bucketRepo, _ = repository.NewBucketRepository(m.DB)
		srcSecretKey := ""
		if cfg != nil {
			srcSecretKey = cfg.AccessSecretKey
		}
		sourceRepo, _ = repository.NewSourceRepository(m.DB, srcSecretKey)
		fileRepo, _ = repository.NewFileRepository(m.DB)
		mpRepo, _ = repository.NewMultipartUploadRepository(m.DB)
		shareLinkRepo, _ = repository.NewShareLinkRepository(m.DB)
	}

	// OAuth and auth utility routes.
	if cfg != nil && userRepo != nil {
		router.GET("/api/v1/auth/github", handlers.GitHubLogin(cfg))
		router.GET("/api/v1/auth/github/callback", handlers.GitHubCallback(cfg, userRepo))
		if auth != nil {
			router.GET("/api/v1/auth/me", auth, handlers.GetCurrentUser(userRepo))
			router.POST("/api/v1/auth/token", auth, handlers.TokenLogin(cfg, userRepo))
		}
	}

	if auth != nil && bucketRepo != nil {
		secretKey := ""
		if cfg != nil {
			secretKey = cfg.AccessSecretKey
		}
		router.POST("/api/v1/buckets", auth, handlers.CreateBucket(bucketRepo, sourceRepo, secretKey))
		router.GET("/api/v1/buckets", auth, handlers.ListBuckets(bucketRepo))
		router.DELETE("/api/v1/buckets/:id", auth, handlers.DeleteBucket(bucketRepo))
		router.PATCH("/api/v1/buckets/:id/distribution", auth, handlers.UpdateBucketDistribution(bucketRepo))
		if sourceRepo != nil && fileRepo != nil {
			chunkSize := 0
			if cfg != nil {
				chunkSize = cfg.Upload.ChunkSize
			}
			router.POST("/api/v1/buckets/:id/upload", auth, handlers.UploadFile(bucketRepo, sourceRepo, fileRepo, chunkSize))
			router.GET("/api/v1/buckets/:id/files", auth, handlers.ListFiles(bucketRepo, fileRepo))
			router.GET("/api/v1/buckets/:id/files/:file_id/download", auth, handlers.DownloadFile(bucketRepo, sourceRepo, fileRepo))
			router.DELETE("/api/v1/buckets/:id/files/:file_id", auth, handlers.DeleteFile(bucketRepo, sourceRepo, fileRepo))
			if shareLinkRepo != nil {
				router.POST("/api/v1/buckets/:id/files/:file_id/share", auth, handlers.CreateShareLink(bucketRepo, fileRepo, shareLinkRepo))
				router.GET("/api/v1/buckets/:id/files/:file_id/shares", auth, handlers.ListShareLinks(bucketRepo, fileRepo, shareLinkRepo))
				router.DELETE("/api/v1/shares/:id", auth, handlers.DeleteShareLink(shareLinkRepo))
			}
		}
	}

	if auth != nil && sourceRepo != nil {
		router.POST("/api/v1/sources/gdrive", auth, handlers.CreateGDriveSource(sourceRepo))
		router.POST("/api/v1/sources/telegram", auth, handlers.CreateTelegramSource(sourceRepo))
		router.POST("/api/v1/sources/s3", auth, handlers.CreateS3Source(sourceRepo))
		router.GET("/api/v1/sources", auth, handlers.ListSources(sourceRepo))
		router.GET("/api/v1/sources/:id/info", auth, handlers.GetSourceInfo(sourceRepo))
		router.GET("/api/v1/sources/:id/files/:file_id/download", auth, handlers.DownloadSourceFile(sourceRepo))
		router.DELETE("/api/v1/sources/:id", auth, handlers.DeleteSource(sourceRepo, bucketRepo))
	}

	// Public share link route (no auth required).
	if shareLinkRepo != nil && bucketRepo != nil && sourceRepo != nil && fileRepo != nil {
		router.GET("/share/:token", handlers.GetSharedFile(shareLinkRepo, bucketRepo, sourceRepo, fileRepo))
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	if bucketRepo != nil && sourceRepo != nil && fileRepo != nil {
		secretKey := ""
		if cfg != nil {
			secretKey = cfg.AccessSecretKey
		}
		chunkSize := 0
		if cfg != nil {
			chunkSize = cfg.Upload.ChunkSize
		}
		s3Auth := handlers.AWS4Auth(bucketRepo, secretKey)
		router.HEAD("/api/s3/:bucket/*object", s3Auth, handlers.HeadObject(bucketRepo, fileRepo))
		router.GET("/api/s3/:bucket/*object", s3Auth, handlers.GetObjectOrParts(bucketRepo, sourceRepo, fileRepo, mpRepo))
		router.GET("/api/s3/:bucket", s3Auth, handlers.ListObjectsOrUploads(bucketRepo, fileRepo, mpRepo))
		router.PUT("/api/s3/:bucket/*object", s3Auth, handlers.PutObjectOrPart(bucketRepo, sourceRepo, fileRepo, mpRepo, chunkSize))
		router.POST("/api/s3/:bucket/*object", s3Auth, handlers.PostObject(bucketRepo, sourceRepo, fileRepo, mpRepo, chunkSize))
		router.DELETE("/api/s3/:bucket/*object", s3Auth, handlers.DeleteObjectOrAbort(bucketRepo, sourceRepo, fileRepo, mpRepo))
	}
	return router
}
