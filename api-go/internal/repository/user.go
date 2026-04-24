package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User represents an authenticated user (basic auth or OAuth).
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Username     string             `bson:"username"`
	PasswordHash string             `bson:"password_hash,omitempty"`
	GitHubID     int64              `bson:"github_id,omitempty"`
	AvatarURL    string             `bson:"avatar_url,omitempty"`
	CreatedAt    time.Time          `bson:"created_at"`
}

type UserRepository struct {
	coll *mongo.Collection
}

func NewUserRepository(ctx context.Context, db *mongo.Database) (*UserRepository, error) {
	coll := db.Collection("users")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, err
	}
	return &UserRepository{coll: coll}, nil
}

func (r *UserRepository) Create(ctx context.Context, user User) (*User, error) {
	user.CreatedAt = user.CreatedAt.UTC()
	res, err := r.coll.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		user.ID = oid
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByGitHubID(ctx context.Context, githubID int64) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, bson.M{"github_id": githubID}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
