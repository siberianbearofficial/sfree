package repository

import (
	"context"
	"regexp"
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
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	BucketID     primitive.ObjectID `bson:"bucket_id"`
	ObjectKey    string             `bson:"object_key"`
	UploadID     string             `bson:"upload_id"`
	Parts        []UploadPart       `bson:"parts"`
	CreatedAt    time.Time          `bson:"created_at"`
	ContentType  string             `bson:"content_type,omitempty"`
	UserMetadata map[string]string  `bson:"user_metadata,omitempty"`
}

type MultipartUploadRepository struct {
	coll *mongo.Collection
}

const multipartPartChunkNameBucketIndex = "parts_chunks_name_bucket_id"
const multipartBucketObjectUploadIndex = "bucket_id_object_key_upload_id"

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
		{
			Keys: bson.D{{Key: "bucket_id", Value: 1}, {Key: "object_key", Value: 1}, {Key: "upload_id", Value: 1}},
			Options: options.Index().
				SetName(multipartBucketObjectUploadIndex),
		},
		{
			Keys: bson.D{{Key: "parts.chunks.name", Value: 1}, {Key: "bucket_id", Value: 1}},
			Options: options.Index().
				SetName(multipartPartChunkNameBucketIndex),
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

func (r *MultipartUploadRepository) ListByBucketPage(ctx context.Context, bucketID primitive.ObjectID, prefix, keyMarker, uploadIDMarker string, limit int) ([]MultipartUpload, bool, error) {
	clauses := []bson.M{{"bucket_id": bucketID}}
	if prefix != "" {
		clauses = append(clauses, bson.M{"object_key": bson.M{"$regex": "^" + regexp.QuoteMeta(prefix)}})
	}
	if keyMarker != "" {
		if uploadIDMarker != "" {
			clauses = append(clauses, bson.M{"$or": bson.A{
				bson.M{"object_key": bson.M{"$gt": keyMarker}},
				bson.M{"object_key": keyMarker, "upload_id": bson.M{"$gt": uploadIDMarker}},
			}})
		} else {
			clauses = append(clauses, bson.M{"object_key": bson.M{"$gt": keyMarker}})
		}
	}

	filter := clauses[0]
	if len(clauses) > 1 {
		filter = bson.M{"$and": clauses}
	}

	findOpts := options.Find().SetSort(bson.D{
		{Key: "object_key", Value: 1},
		{Key: "upload_id", Value: 1},
	})
	if limit >= 0 {
		findOpts.SetLimit(int64(limit + 1))
	}
	cursor, err := r.coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	uploads := make([]MultipartUpload, 0, max(limit, 0))
	for cursor.Next(ctx) {
		var mu MultipartUpload
		if err := cursor.Decode(&mu); err != nil {
			return nil, false, err
		}
		uploads = append(uploads, mu)
	}
	if err := cursor.Err(); err != nil {
		return nil, false, err
	}

	hasMore := limit >= 0 && len(uploads) > limit
	if hasMore {
		uploads = uploads[:limit]
	}
	return uploads, hasMore, nil
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
