package main

import (
	"log"

	"github.com/example/s3aas/api-go/internal/app"
)

func main() {
	router := app.SetupRouter()
	if err := router.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
