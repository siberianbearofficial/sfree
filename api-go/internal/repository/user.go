package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User represents basic auth user
type User struct {
	Username     string `bson:"username"`
	PasswordHash string `bson:"password_hash"`
}

type UserRepository struct {
	coll *mongo.Collection
}

func NewUserRepository(db *mongo.Database) (*UserRepository, error) {
	coll := db.Collection("users")
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, err
	}
	return &UserRepository{coll: coll}, nil
}

func (r *UserRepository) Create(ctx context.Context, user User) error {
	_, err := r.coll.InsertOne(ctx, user)
	return err
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
