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

type FileChunk struct {
	SourceID primitive.ObjectID `bson:"source_id"`
	Name     string             `bson:"name"`
	Order    int                `bson:"order"`
	Size     int64              `bson:"size"`
	// Checksum is the hex-encoded SHA-256 hash of the raw chunk bytes, computed
	// at upload time. Empty for chunks created before checksums were introduced.
	Checksum string `bson:"checksum,omitempty"`
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

const fileBucketNameUniqueIndex = "bucket_id_name_unique"

func NewFileRepository(db *mongo.Database) (*FileRepository, error) {
	coll := db.Collection("files")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "bucket_id", Value: 1}},
	})
	if err != nil {
		return nil, err
	}
	if err := ensureUniqueFileBucketNameIndex(context.Background(), coll); err != nil {
		return nil, err
	}
	return &FileRepository{coll: coll}, nil
}

type indexInfo struct {
	Name   string `bson:"name"`
	Key    bson.D `bson:"key"`
	Unique bool   `bson:"unique,omitempty"`
}

func ensureUniqueFileBucketNameIndex(ctx context.Context, coll *mongo.Collection) error {
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = cursor.Close(ctx) }()

	hasUnique := false
	var dropNames []string
	for cursor.Next(ctx) {
		var idx indexInfo
		if err := cursor.Decode(&idx); err != nil {
			return err
		}
		if !isFileBucketNameIndex(idx.Key) {
			continue
		}
		if idx.Unique {
			hasUnique = true
			continue
		}
		dropNames = append(dropNames, idx.Name)
	}
	if err := cursor.Err(); err != nil {
		return err
	}
	for _, name := range dropNames {
		if _, err := coll.Indexes().DropOne(ctx, name); err != nil {
			return err
		}
	}
	if hasUnique {
		return nil
	}
	_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "bucket_id", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().
			SetName(fileBucketNameUniqueIndex).
			SetUnique(true),
	})
	return err
}

func isFileBucketNameIndex(key bson.D) bool {
	return len(key) == 2 &&
		key[0].Key == "bucket_id" &&
		key[1].Key == "name" &&
		indexValueIsOne(key[0].Value) &&
		indexValueIsOne(key[1].Value)
}

func indexValueIsOne(value any) bool {
	switch v := value.(type) {
	case int:
		return v == 1
	case int32:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return v == 1
	default:
		return false
	}
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

func (r *FileRepository) GetByName(ctx context.Context, bucketID primitive.ObjectID, name string) (*File, error) {
	var f File
	err := r.coll.FindOne(ctx, bson.M{"bucket_id": bucketID, "name": name}).Decode(&f)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FileRepository) CountByChunk(ctx context.Context, sourceID primitive.ObjectID, name string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{
		"chunks": bson.M{"$elemMatch": bson.M{
			"source_id": sourceID,
			"name":      name,
		}},
	})
}

func (r *FileRepository) ListByBucket(ctx context.Context, bucketID primitive.ObjectID) ([]File, error) {
	return r.ListByBucketWithPrefix(ctx, bucketID, "")
}

func (r *FileRepository) ListByBucketWithPrefix(ctx context.Context, bucketID primitive.ObjectID, prefix string) ([]File, error) {
	filter := bson.M{"bucket_id": bucketID}
	if prefix != "" {
		filter["name"] = bson.M{"$regex": "^" + regexp.QuoteMeta(prefix)}
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var files []File
	for cursor.Next(ctx) {
		var f File
		if err := cursor.Decode(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (r *FileRepository) ListByBucketWithPrefixPage(ctx context.Context, bucketID primitive.ObjectID, prefix, after string, limit int) ([]File, bool, error) {
	filter := bson.M{"bucket_id": bucketID}
	nameFilter := bson.M{}
	if prefix != "" {
		nameFilter["$regex"] = "^" + regexp.QuoteMeta(prefix)
	}
	if after != "" {
		nameFilter["$gt"] = after
	}
	if len(nameFilter) > 0 {
		filter["name"] = nameFilter
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	if limit >= 0 {
		findOpts.SetLimit(int64(limit + 1))
	}
	cursor, err := r.coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	files := make([]File, 0, limit)
	for cursor.Next(ctx) {
		var f File
		if err := cursor.Decode(&f); err != nil {
			return nil, false, err
		}
		files = append(files, f)
	}
	if err := cursor.Err(); err != nil {
		return nil, false, err
	}

	hasMore := limit >= 0 && len(files) > limit
	if hasMore {
		files = files[:limit]
	}
	return files, hasMore, nil
}

func (r *FileRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *FileRepository) DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"bucket_id": bucketID})
	return err
}

func (r *FileRepository) UpdateByID(ctx context.Context, f File) (*File, error) {
	f.CreatedAt = f.CreatedAt.UTC()
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": f.ID}, bson.M{"$set": bson.M{
		"bucket_id":  f.BucketID,
		"name":       f.Name,
		"created_at": f.CreatedAt,
		"chunks":     f.Chunks,
	}})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, mongo.ErrNoDocuments
	}
	return &f, nil
}

func (r *FileRepository) ReplaceByName(ctx context.Context, f File) (*File, *File, error) {
	f.CreatedAt = f.CreatedAt.UTC()
	if f.ID.IsZero() {
		f.ID = primitive.NewObjectID()
	}
	current := f

	previous, err := r.replaceByName(ctx, current, true)
	if err == nil {
		current.ID = previous.ID
		return &current, previous, nil
	}
	if err == mongo.ErrNoDocuments {
		return &current, nil, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return nil, nil, err
	}

	previous, err = r.replaceByName(ctx, current, false)
	if err != nil {
		return nil, nil, err
	}
	current.ID = previous.ID
	return &current, previous, nil
}

func (r *FileRepository) replaceByName(ctx context.Context, f File, upsert bool) (*File, error) {
	update := bson.M{
		"$set": bson.M{
			"bucket_id":  f.BucketID,
			"name":       f.Name,
			"created_at": f.CreatedAt,
			"chunks":     f.Chunks,
		},
	}
	if upsert {
		update["$setOnInsert"] = bson.M{"_id": f.ID}
	}

	var previous File
	err := r.coll.FindOneAndUpdate(
		ctx,
		bson.M{"bucket_id": f.BucketID, "name": f.Name},
		update,
		options.FindOneAndUpdate().
			SetReturnDocument(options.Before).
			SetUpsert(upsert),
	).Decode(&previous)
	if err != nil {
		return nil, err
	}
	return &previous, nil
}
