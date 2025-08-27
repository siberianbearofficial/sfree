package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Bucket struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Key          string             `bson:"key"`
	AccessKey    string             `bson:"access_key"`
	AccessSecret string             `bson:"access_secret"`
	CreatedAt    time.Time          `bson:"created_at"`
}

type BucketRepository struct {
	coll *mongo.Collection
}

func NewBucketRepository(db *mongo.Database) (*BucketRepository, error) {
	coll := db.Collection("buckets")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "key", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, err
	}
	return &BucketRepository{coll: coll}, nil
}

func (r *BucketRepository) Create(ctx context.Context, b Bucket) (*Bucket, error) {
	b.CreatedAt = b.CreatedAt.UTC()
	res, err := r.coll.InsertOne(ctx, b)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		b.ID = oid
	}
	return &b, nil
}

func (r *BucketRepository) GetByKey(ctx context.Context, key string) (*Bucket, error) {
	var b Bucket
	err := r.coll.FindOne(ctx, bson.M{"key": key}).Decode(&b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BucketRepository) List(ctx context.Context) ([]Bucket, error) {
	cursor, err := r.coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var buckets []Bucket
	for cursor.Next(ctx) {
		var b Bucket
		if err := cursor.Decode(&b); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return buckets, nil
}

func (r *BucketRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
