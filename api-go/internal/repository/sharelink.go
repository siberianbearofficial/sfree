package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ShareLink struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	FileID    primitive.ObjectID `bson:"file_id"`
	BucketID  primitive.ObjectID `bson:"bucket_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Token     string             `bson:"token"`
	ExpiresAt *time.Time         `bson:"expires_at,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
}

type ShareLinkRepository struct {
	coll *mongo.Collection
}

func NewShareLinkRepository(db *mongo.Database) (*ShareLinkRepository, error) {
	coll := db.Collection("share_links")
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "file_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "bucket_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
	})
	if err != nil {
		return nil, err
	}
	return &ShareLinkRepository{coll: coll}, nil
}

func (r *ShareLinkRepository) Create(ctx context.Context, sl ShareLink) (*ShareLink, error) {
	sl.CreatedAt = sl.CreatedAt.UTC()
	if sl.ExpiresAt != nil {
		t := sl.ExpiresAt.UTC()
		sl.ExpiresAt = &t
	}
	res, err := r.coll.InsertOne(ctx, sl)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		sl.ID = oid
	}
	return &sl, nil
}

func (r *ShareLinkRepository) GetByToken(ctx context.Context, token string) (*ShareLink, error) {
	var sl ShareLink
	err := r.coll.FindOne(ctx, bson.M{"token": token}).Decode(&sl)
	if err != nil {
		return nil, err
	}
	return &sl, nil
}

func (r *ShareLinkRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*ShareLink, error) {
	var sl ShareLink
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&sl)
	if err != nil {
		return nil, err
	}
	return &sl, nil
}

func (r *ShareLinkRepository) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]ShareLink, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var links []ShareLink
	for cursor.Next(ctx) {
		var sl ShareLink
		if err := cursor.Decode(&sl); err != nil {
			return nil, err
		}
		links = append(links, sl)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

func (r *ShareLinkRepository) ListByFile(ctx context.Context, fileID primitive.ObjectID) ([]ShareLink, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"file_id": fileID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var links []ShareLink
	for cursor.Next(ctx) {
		var sl ShareLink
		if err := cursor.Decode(&sl); err != nil {
			return nil, err
		}
		links = append(links, sl)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

func (r *ShareLinkRepository) Delete(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *ShareLinkRepository) DeleteByFile(ctx context.Context, fileID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"file_id": fileID})
	return err
}

func (r *ShareLinkRepository) DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"bucket_id": bucketID})
	return err
}
