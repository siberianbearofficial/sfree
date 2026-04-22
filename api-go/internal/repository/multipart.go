package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UploadPart struct {
	PartNumber int         `bson:"part_number"`
	ETag       string      `bson:"etag"`
	Size       int64       `bson:"size"`
	Chunks     []FileChunk `bson:"chunks"`
}

type MultipartUpload struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	BucketID  primitive.ObjectID `bson:"bucket_id"`
	ObjectKey string             `bson:"object_key"`
	UploadID  string             `bson:"upload_id"`
	Parts     []UploadPart       `bson:"parts"`
	CreatedAt time.Time          `bson:"created_at"`
}

type MultipartUploadRepository struct {
	coll *mongo.Collection
}

func NewMultipartUploadRepository(db *mongo.Database) (*MultipartUploadRepository, error) {
	coll := db.Collection("multipart_uploads")
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "upload_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "bucket_id", Value: 1}},
		},
	})
	if err != nil {
		return nil, err
	}
	return &MultipartUploadRepository{coll: coll}, nil
}

func (r *MultipartUploadRepository) Create(ctx context.Context, mu MultipartUpload) (*MultipartUpload, error) {
	mu.CreatedAt = mu.CreatedAt.UTC()
	if mu.Parts == nil {
		mu.Parts = []UploadPart{}
	}
	res, err := r.coll.InsertOne(ctx, mu)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		mu.ID = oid
	}
	return &mu, nil
}

func (r *MultipartUploadRepository) GetByUploadID(ctx context.Context, uploadID string) (*MultipartUpload, error) {
	var mu MultipartUpload
	err := r.coll.FindOne(ctx, bson.M{"upload_id": uploadID}).Decode(&mu)
	if err != nil {
		return nil, err
	}
	return &mu, nil
}

func (r *MultipartUploadRepository) ListByBucket(ctx context.Context, bucketID primitive.ObjectID) ([]MultipartUpload, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"bucket_id": bucketID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var uploads []MultipartUpload
	for cursor.Next(ctx) {
		var mu MultipartUpload
		if err := cursor.Decode(&mu); err != nil {
			return nil, err
		}
		uploads = append(uploads, mu)
	}
	return uploads, cursor.Err()
}

func (r *MultipartUploadRepository) CountByPartChunk(ctx context.Context, sourceID primitive.ObjectID, name string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{
		"parts": bson.M{"$elemMatch": bson.M{
			"chunks": bson.M{"$elemMatch": bson.M{
				"source_id": sourceID,
				"name":      name,
			}},
		}},
	})
}

func (r *MultipartUploadRepository) CountByPartChunkExcludingBucket(ctx context.Context, bucketID, sourceID primitive.ObjectID, name string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{
		"bucket_id": bson.M{"$ne": bucketID},
		"parts": bson.M{"$elemMatch": bson.M{
			"chunks": bson.M{"$elemMatch": bson.M{
				"source_id": sourceID,
				"name":      name,
			}},
		}},
	})
}

func (r *MultipartUploadRepository) SetPart(ctx context.Context, uploadID string, part UploadPart) (*UploadPart, error) {
	update := mongo.Pipeline{
		{{
			Key: "$set",
			Value: bson.D{{
				Key: "parts",
				Value: bson.D{{
					Key: "$concatArrays",
					Value: bson.A{
						bson.D{{
							Key: "$filter",
							Value: bson.D{
								{Key: "input", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$parts", bson.A{}}}}},
								{Key: "as", Value: "part"},
								{Key: "cond", Value: bson.D{{Key: "$ne", Value: bson.A{"$$part.part_number", part.PartNumber}}}},
							},
						}},
						bson.A{part},
					},
				}},
			}},
		}},
	}
	var previousUpload MultipartUpload
	if err := r.coll.FindOneAndUpdate(ctx, bson.M{"upload_id": uploadID}, update).Decode(&previousUpload); err != nil {
		return nil, err
	}
	for _, existing := range previousUpload.Parts {
		if existing.PartNumber == part.PartNumber {
			partCopy := existing
			return &partCopy, nil
		}
	}
	return nil, nil
}

func (r *MultipartUploadRepository) DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"bucket_id": bucketID})
	return err
}

func (r *MultipartUploadRepository) Delete(ctx context.Context, uploadID string) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"upload_id": uploadID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
