package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BucketRole string

const (
	RoleOwner  BucketRole = "owner"
	RoleEditor BucketRole = "editor"
	RoleViewer BucketRole = "viewer"
)

// ValidRole returns true if the role is a recognised bucket role.
func ValidRole(r BucketRole) bool {
	return r == RoleOwner || r == RoleEditor || r == RoleViewer
}

// RoleAtLeast returns true when have >= required in the hierarchy owner > editor > viewer.
func RoleAtLeast(have, required BucketRole) bool {
	return roleRank(have) >= roleRank(required)
}

func roleRank(r BucketRole) int {
	switch r {
	case RoleOwner:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

type BucketGrant struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	BucketID  primitive.ObjectID `bson:"bucket_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Role      BucketRole         `bson:"role"`
	GrantedBy primitive.ObjectID `bson:"granted_by"`
	CreatedAt time.Time          `bson:"created_at"`
}

type BucketGrantRepository struct {
	coll *mongo.Collection
}

func NewBucketGrantRepository(db *mongo.Database) (*BucketGrantRepository, error) {
	coll := db.Collection("bucket_grants")
	_, err := coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "bucket_id", Value: 1}, {Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "bucket_id", Value: 1}},
		},
	})
	if err != nil {
		return nil, err
	}
	return &BucketGrantRepository{coll: coll}, nil
}

func (r *BucketGrantRepository) Create(ctx context.Context, g BucketGrant) (*BucketGrant, error) {
	g.CreatedAt = g.CreatedAt.UTC()
	res, err := r.coll.InsertOne(ctx, g)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		g.ID = oid
	}
	return &g, nil
}

func (r *BucketGrantRepository) GetByBucketAndUser(ctx context.Context, bucketID, userID primitive.ObjectID) (*BucketGrant, error) {
	var g BucketGrant
	err := r.coll.FindOne(ctx, bson.M{"bucket_id": bucketID, "user_id": userID}).Decode(&g)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *BucketGrantRepository) ListByBucket(ctx context.Context, bucketID primitive.ObjectID) ([]BucketGrant, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"bucket_id": bucketID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var grants []BucketGrant
	for cursor.Next(ctx) {
		var g BucketGrant
		if err := cursor.Decode(&g); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, cursor.Err()
}

func (r *BucketGrantRepository) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]BucketGrant, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	var grants []BucketGrant
	for cursor.Next(ctx) {
		var g BucketGrant
		if err := cursor.Decode(&g); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, cursor.Err()
}

func (r *BucketGrantRepository) UpdateRole(ctx context.Context, bucketID, id primitive.ObjectID, role BucketRole) error {
	res, err := r.coll.UpdateOne(ctx, bucketGrantDocumentFilter(bucketID, id), bson.M{"$set": bson.M{"role": role}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *BucketGrantRepository) Delete(ctx context.Context, bucketID, id primitive.ObjectID) error {
	res, err := r.coll.DeleteOne(ctx, bucketGrantDocumentFilter(bucketID, id))
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *BucketGrantRepository) DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"bucket_id": bucketID})
	return err
}

func bucketGrantDocumentFilter(bucketID, id primitive.ObjectID) bson.M {
	return bson.M{"bucket_id": bucketID, "_id": id}
}
