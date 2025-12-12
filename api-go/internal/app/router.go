package app

import (
	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
	"github.com/example/s3aas/api-go/internal/handlers"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(m *db.Mongo, cfg *config.Config) *gin.Engine {
	router := gin.New()
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	router.Use(gin.Recovery())
	router.GET("/readyz", handlers.Readyz)
	router.GET("/healthz", handlers.Healthz)
	router.GET("/publication/ready", handlers.PublicationReady)
	router.GET("/dbz", handlers.DBProbe(m))

	var (
		auth       gin.HandlerFunc
		bucketRepo *repository.BucketRepository
		sourceRepo *repository.SourceRepository
		fileRepo   *repository.FileRepository
	)

	if m != nil {
		if userRepo, err := repository.NewUserRepository(m.DB); err == nil {
			auth = handlers.BasicAuth(userRepo)
			router.POST("/api/v1/users", handlers.CreateUser(userRepo))
		}
		bucketRepo, _ = repository.NewBucketRepository(m.DB)
		sourceRepo, _ = repository.NewSourceRepository(m.DB)
		fileRepo, _ = repository.NewFileRepository(m.DB)
	}

	if auth != nil && bucketRepo != nil {
		secretKey := ""
		if cfg != nil {
			secretKey = cfg.AccessSecretKey
		}
		router.POST("/api/v1/buckets", auth, handlers.CreateBucket(bucketRepo, secretKey))
		router.GET("/api/v1/buckets", auth, handlers.ListBuckets(bucketRepo))
		router.DELETE("/api/v1/buckets/:id", auth, handlers.DeleteBucket(bucketRepo))
		if sourceRepo != nil && fileRepo != nil {
			chunkSize := 0
			if cfg != nil {
				chunkSize = cfg.Upload.ChunkSize
			}
			router.POST("/api/v1/buckets/:id/upload", auth, handlers.UploadFile(bucketRepo, sourceRepo, fileRepo, chunkSize))
			router.GET("/api/v1/buckets/:id/files", auth, handlers.ListFiles(bucketRepo, fileRepo))
			router.GET("/api/v1/buckets/:id/files/:file_id/download", auth, handlers.DownloadFile(bucketRepo, sourceRepo, fileRepo))
			router.DELETE("/api/v1/buckets/:id/files/:file_id", auth, handlers.DeleteFile(bucketRepo, sourceRepo, fileRepo))
		}
	}

	if auth != nil && sourceRepo != nil {
		router.POST("/api/v1/sources/gdrive", auth, handlers.CreateGDriveSource(sourceRepo))
		router.GET("/api/v1/sources", auth, handlers.ListSources(sourceRepo))
		router.GET("/api/v1/sources/:id/info", auth, handlers.GetSourceInfo(sourceRepo))
		router.DELETE("/api/v1/sources/:id", auth, handlers.DeleteSource(sourceRepo))
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	if bucketRepo != nil && sourceRepo != nil && fileRepo != nil {
		secretKey := ""
		if cfg != nil {
			secretKey = cfg.AccessSecretKey
		}
		router.GET("/api/s3/:bucket/*object", handlers.AWS4Auth(bucketRepo, secretKey), handlers.GetObject(bucketRepo, sourceRepo, fileRepo))
	}
	return router
}
