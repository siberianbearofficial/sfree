package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SourceType represents type of external source.
type SourceType string

const (
	// SourceTypeGDrive is Google Drive source type.
	SourceTypeGDrive SourceType = "gdrive"
)

// Source represents external data source stored in MongoDB.
type Source struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Type      SourceType         `bson:"type"`
	Name      string             `bson:"name"`
	Key       string             `bson:"key"`
	CreatedAt time.Time          `bson:"created_at"`
}

// SourceRepository handles CRUD operations on sources.
type SourceRepository struct {
	coll *mongo.Collection
}

// NewSourceRepository creates new SourceRepository.
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

// Create stores new source in MongoDB.
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

// GetByID returns source by its ID.
func (r *SourceRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*Source, error) {
	var s Source
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
