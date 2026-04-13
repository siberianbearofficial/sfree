// Command migrate-source-keys encrypts existing plaintext source credentials
// in MongoDB using AES-256-GCM. It reads the ACCESS_SECRET_KEY env var (or
// config file) to derive the encryption key, then iterates over every document
// in the sources collection. Records that are already encrypted (valid
// base64 + successful AES-GCM open) are skipped.
//
// Usage:
//
//	ACCESS_SECRET_KEY=<key> go run ./cmd/migrate-source-keys
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	secretKey := cfg.AccessSecretKey
	if secretKey == "" {
		fmt.Fprintln(os.Stderr, "ACCESS_SECRET_KEY is not configured")
		os.Exit(1)
	}

	ctx := context.Background()
	mongo, err := db.Connect(ctx, cfg.Mongo)
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer func() { _ = mongo.Client.Disconnect(ctx) }()

	coll := mongo.DB.Collection("sources")
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("find sources: %v", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var migrated, skipped, total int
	for cursor.Next(ctx) {
		total++
		var doc struct {
			ID  primitive.ObjectID `bson:"_id"`
			Key string             `bson:"key"`
		}
		if err := cursor.Decode(&doc); err != nil {
			log.Fatalf("decode source: %v", err)
		}
		// Try decrypting — if it succeeds, the record is already encrypted.
		if _, err := cryptoutil.Decrypt(doc.Key, secretKey); err == nil {
			skipped++
			continue
		}
		encrypted, err := cryptoutil.Encrypt(doc.Key, secretKey)
		if err != nil {
			log.Fatalf("encrypt source %s: %v", doc.ID.Hex(), err)
		}
		_, err = coll.UpdateByID(ctx, doc.ID, bson.M{"$set": bson.M{"key": encrypted}})
		if err != nil {
			log.Fatalf("update source %s: %v", doc.ID.Hex(), err)
		}
		migrated++
	}
	if err := cursor.Err(); err != nil {
		log.Fatalf("cursor error: %v", err)
	}
	fmt.Printf("migration complete: %d total, %d migrated, %d already encrypted\n", total, migrated, skipped)
}
