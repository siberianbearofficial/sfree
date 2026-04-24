package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrSourcesNotFound = errors.New("sources not found")

type SourcesNotFoundError struct {
	IDs []primitive.ObjectID
}

func (e SourcesNotFoundError) Error() string {
	if len(e.IDs) == 0 {
		return ErrSourcesNotFound.Error()
	}
	ids := make([]string, 0, len(e.IDs))
	for _, id := range e.IDs {
		ids = append(ids, id.Hex())
	}
	return fmt.Sprintf("%s: %s", ErrSourcesNotFound, strings.Join(ids, ", "))
}

func (e SourcesNotFoundError) Unwrap() error {
	return ErrSourcesNotFound
}

type SourceType string

const (
	SourceTypeGDrive   SourceType = "gdrive"
	SourceTypeTelegram SourceType = "telegram"
	SourceTypeS3       SourceType = "s3"
)

type Source struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Type      SourceType         `bson:"type"`
	Name      string             `bson:"name"`
	Key       string             `bson:"key"`
	CreatedAt time.Time          `bson:"created_at"`
}

type SourceRepository struct {
	coll      *mongo.Collection
	secretKey string
}

func NewSourceRepository(ctx context.Context, db *mongo.Database, secretKey ...string) (*SourceRepository, error) {
	coll := db.Collection("sources")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	key := ""
	if len(secretKey) > 0 {
		key = secretKey[0]
	}
	return &SourceRepository{coll: coll, secretKey: key}, nil
}

func (r *SourceRepository) encryptKey(plain string) (string, error) {
	if r.secretKey == "" {
		return plain, nil
	}
	return cryptoutil.Encrypt(plain, r.secretKey)
}

func (r *SourceRepository) decryptKey(cipher string) (string, error) {
	if r.secretKey == "" || cipher == "" {
		return cipher, nil
	}
	plain, err := cryptoutil.Decrypt(cipher, r.secretKey)
	if err != nil {
		return "", fmt.Errorf("decrypt source key: %w", err)
	}
	return plain, nil
}

func (r *SourceRepository) Create(ctx context.Context, s Source) (*Source, error) {
	s.CreatedAt = s.CreatedAt.UTC()
	enc, err := r.encryptKey(s.Key)
	if err != nil {
		return nil, err
	}
	plainKey := s.Key
	s.Key = enc
	res, err := r.coll.InsertOne(ctx, s)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		s.ID = oid
	}
	s.Key = plainKey
	return &s, nil
}

func (r *SourceRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*Source, error) {
	var s Source
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		return nil, err
	}
	s.Key, err = r.decryptKey(s.Key)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SourceRepository) ListByIDs(ctx context.Context, ids []primitive.ObjectID) ([]Source, error) {
	if len(ids) == 0 {
		return []Source{}, nil
	}
	cursor, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	byID := make(map[primitive.ObjectID]Source, len(ids))
	for cursor.Next(ctx) {
		var src Source
		if err := cursor.Decode(&src); err != nil {
			return nil, err
		}
		src.Key, err = r.decryptKey(src.Key)
		if err != nil {
			return nil, err
		}
		byID[src.ID] = src
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return orderSourcesByIDs(ids, byID)
}

func orderSourcesByIDs(ids []primitive.ObjectID, byID map[primitive.ObjectID]Source) ([]Source, error) {
	if len(ids) == 0 {
		return []Source{}, nil
	}
	sources := make([]Source, 0, len(ids))
	missing := make([]primitive.ObjectID, 0)
	for _, id := range ids {
		source, ok := byID[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		sources = append(sources, source)
	}
	if len(missing) > 0 {
		return nil, SourcesNotFoundError{IDs: missing}
	}
	return sources, nil
}

func (r *SourceRepository) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]Source, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var sources []Source
	for cursor.Next(ctx) {
		var s Source
		if err := cursor.Decode(&s); err != nil {
			return nil, err
		}
		s.Key, err = r.decryptKey(s.Key)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *SourceRepository) ListMetadataByUser(ctx context.Context, userID primitive.ObjectID) ([]Source, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID}, options.Find().SetProjection(bson.M{"key": 0}))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var sources []Source
	for cursor.Next(ctx) {
		var s Source
		if err := cursor.Decode(&s); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *SourceRepository) Delete(ctx context.Context, id, userID primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
