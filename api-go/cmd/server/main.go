// Package main implements the HTTP server.
//
// @title S3aaS API
// @version 1.0
// @BasePath /
// @securityDefinitions.basic BasicAuth
package main

import (
	"context"
	"log"

	"github.com/example/s3aas/api-go/internal/app"
	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
	_ "github.com/example/s3aas/api-go/internal/docs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	ctx := context.Background()
	mongoConn, err := db.Connect(ctx, cfg.Mongo)
	if err != nil {
		log.Fatalf("failed to connect mongo: %v", err)
	}
	router := app.SetupRouter(mongoConn, cfg)
	if err := router.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
