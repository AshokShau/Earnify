package main

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID         int64   `bson:"_id,omitempty" json:"_id,omitempty"`
	ReferredBy int64   `bson:"referred_by,omitempty" json:"referred_by,omitempty"`
	ReferredTo []int64 `bson:"referred_to,omitempty" json:"referred_to,omitempty"`
	AccNo      int64   `bson:"acc_no,omitempty" json:"acc_no,omitempty"`
	Balance    float64 `bson:"balance,omitempty" json:"balance,omitempty"`
}

var (
	userColl *mongo.Collection
	ctx      = context.TODO()
)

func connectDB() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	db := client.Database("tgReferEarn")
	userColl = db.Collection("users")
}

func findOne(collection *mongo.Collection, filter bson.M) *mongo.SingleResult {
	return collection.FindOne(ctx, filter)
}
