// Package main implements the HTTP server.
//
// @title S3aaS API
// @version 1.0
// @BasePath /
package main

import (
	"context"
	"log"

	"github.com/example/s3aas/api-go/internal/app"
	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
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
	router := app.SetupRouter(mongoConn)
	if err := router.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
