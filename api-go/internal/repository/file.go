package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileChunk struct {
	SourceID primitive.ObjectID `bson:"source_id"`
	Name     string             `bson:"name"`
	Order    int                `bson:"order"`
	Size     int64              `bson:"size"`
}

type File struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	BucketID  primitive.ObjectID `bson:"bucket_id"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"created_at"`
	Chunks    []FileChunk        `bson:"chunks"`
}

type FileRepository struct {
	coll *mongo.Collection
}

func NewFileRepository(db *mongo.Database) (*FileRepository, error) {
	coll := db.Collection("files")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "bucket_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	return &FileRepository{coll: coll}, nil
}

func (r *FileRepository) Create(ctx context.Context, f File) (*File, error) {
	f.CreatedAt = f.CreatedAt.UTC()
	res, err := r.coll.InsertOne(ctx, f)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		f.ID = oid
	}
	return &f, nil
}

func (r *FileRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*File, error) {
	var f File
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&f)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
