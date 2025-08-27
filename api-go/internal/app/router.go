package app

import (
	"github.com/example/s3aas/api-go/internal/db"
	"github.com/example/s3aas/api-go/internal/handlers"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(m *db.Mongo) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/readyz", handlers.Readyz)
	router.GET("/healthz", handlers.Healthz)
	router.GET("/publication/ready", handlers.PublicationReady)
	router.GET("/dbz", handlers.DBProbe(m))
	if m != nil {
		if userRepo, err := repository.NewUserRepository(m.DB); err == nil {
			router.POST("/api/v1/users", handlers.CreateUser(userRepo))
		}
		if bucketRepo, err := repository.NewBucketRepository(m.DB); err == nil {
			router.POST("/api/v1/buckets", handlers.CreateBucket(bucketRepo))
		}
		if sourceRepo, err := repository.NewSourceRepository(m.DB); err == nil {
			router.POST("/api/v1/sources/gdrive", handlers.CreateGDriveSource(sourceRepo))
		}
	}
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	return router
}
