package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DistributionStrategy string

const (
	StrategyRoundRobin DistributionStrategy = "round_robin"
	StrategyWeighted   DistributionStrategy = "weighted"
)

type Bucket struct {
	ID                   primitive.ObjectID   `bson:"_id,omitempty"`
	UserID               primitive.ObjectID   `bson:"user_id"`
	Key                  string               `bson:"key"`
	AccessKey            string               `bson:"access_key"`
	AccessSecretEnc      string               `bson:"access_secret"`
	SourceIDs            []primitive.ObjectID `bson:"source_ids"`
	DistributionStrategy DistributionStrategy `bson:"distribution_strategy,omitempty"`
	SourceWeights        map[string]int       `bson:"source_weights,omitempty"`
	CreatedAt            time.Time            `bson:"created_at"`
}

type BucketRepository struct {
	coll *mongo.Collection
}

func NewBucketRepository(db *mongo.Database) (*BucketRepository, error) {
	coll := db.Collection("buckets")
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "access_key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
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

func (r *BucketRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*Bucket, error) {
	var b Bucket
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&b)
	if err != nil {
		return nil, err
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

func (r *BucketRepository) GetByAccessKey(ctx context.Context, accessKey string) (*Bucket, error) {
	var b Bucket
	err := r.coll.FindOne(ctx, bson.M{"access_key": accessKey}).Decode(&b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BucketRepository) GetSecretByAccessKey(ctx context.Context, accessKey string) (string, error) {
	var b Bucket
	err := r.coll.FindOne(ctx, bson.M{"access_key": accessKey}).Decode(&b)
	if err != nil {
		return "", err
	}
	return b.AccessSecretEnc, nil
}

func (r *BucketRepository) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]Bucket, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
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

func (r *BucketRepository) ListByIDs(ctx context.Context, ids []primitive.ObjectID) ([]Bucket, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cursor, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
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
	return buckets, cursor.Err()
}

func (r *BucketRepository) HasSourceReference(ctx context.Context, userID, sourceID primitive.ObjectID) (bool, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{"user_id": userID, "source_ids": sourceID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *BucketRepository) UpdateDistribution(ctx context.Context, id, userID primitive.ObjectID, strategy DistributionStrategy, weights map[string]int) error {
	update := bson.M{"$set": bson.M{
		"distribution_strategy": strategy,
		"source_weights":        weights,
	}}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": id, "user_id": userID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *BucketRepository) Delete(ctx context.Context, id, userID primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
