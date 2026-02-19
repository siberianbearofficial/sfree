package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

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
	coll *mongo.Collection
}

func NewSourceRepository(db *mongo.Database) (*SourceRepository, error) {
	coll := db.Collection("sources")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	return &SourceRepository{coll: coll}, nil
}

func (r *SourceRepository) Create(ctx context.Context, s Source) (*Source, error) {
	s.CreatedAt = s.CreatedAt.UTC()
	res, err := r.coll.InsertOne(ctx, s)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		s.ID = oid
	}
	return &s, nil
}

func (r *SourceRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*Source, error) {
	var s Source
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
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
