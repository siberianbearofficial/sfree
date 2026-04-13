// Command migrate-source-keys encrypts existing plaintext source credentials
// in MongoDB using AES-256-GCM. It reads the ACCESS_SECRET_KEY env var (or
// config file) to derive the encryption key, then iterates over every document
// in the sources collection. Records that are already encrypted (valid
// base64 + successful AES-GCM open) are skipped.
//
// Usage:
//
//	ACCESS_SECRET_KEY=<key> go run ./cmd/migrate-source-keys
//
// If the key was rotated, pass the old key via OLD_SECRET_KEY to re-encrypt:
//
//	ACCESS_SECRET_KEY=<new> OLD_SECRET_KEY=<old> go run ./cmd/migrate-source-keys
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// looksLikeCiphertext checks whether a value looks structurally like
// AES-GCM ciphertext (valid base64, at least nonce+1 bytes). This is
// used to avoid encrypting data that might already be encrypted with a
// different key, which would produce permanently unusable double-encrypted
// values.
func looksLikeCiphertext(s string) bool {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	// AES-GCM nonce is 12 bytes; ciphertext must be at least nonce + 1 byte.
	return len(data) > 12
}

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

	// Optional: old key for key-rotation re-encryption.
	oldKey := os.Getenv("OLD_SECRET_KEY")

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

	var migrated, reencrypted, skipped, errored, total int
	for cursor.Next(ctx) {
		total++
		var doc struct {
			ID  primitive.ObjectID `bson:"_id"`
			Key string             `bson:"key"`
		}
		if err := cursor.Decode(&doc); err != nil {
			log.Fatalf("decode source: %v", err)
		}

		// Already encrypted with current key — skip.
		if _, err := cryptoutil.Decrypt(doc.Key, secretKey); err == nil {
			skipped++
			continue
		}

		// Try old key for re-encryption (key rotation).
		if oldKey != "" {
			if plain, err := cryptoutil.Decrypt(doc.Key, oldKey); err == nil {
				enc, err := cryptoutil.Encrypt(plain, secretKey)
				if err != nil {
					log.Fatalf("re-encrypt source %s: %v", doc.ID.Hex(), err)
				}
				if _, err := coll.UpdateByID(ctx, doc.ID, bson.M{"$set": bson.M{"key": enc}}); err != nil {
					log.Fatalf("update source %s: %v", doc.ID.Hex(), err)
				}
				reencrypted++
				continue
			}
		}

		// Guard against double-encryption: if the value looks like
		// ciphertext but could not be decrypted with any known key,
		// refuse to encrypt it — it may be encrypted with a rotated
		// key that was not provided.
		if looksLikeCiphertext(doc.Key) {
			log.Printf("WARNING: source %s has ciphertext-like data that cannot be decrypted; skipping to avoid double-encryption (provide OLD_SECRET_KEY if key was rotated)", doc.ID.Hex())
			errored++
			continue
		}

		// Plaintext — encrypt it.
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
	fmt.Printf("migration complete: %d total, %d migrated, %d re-encrypted, %d already encrypted, %d skipped (undecryptable)\n",
		total, migrated, reencrypted, skipped, errored)
}
