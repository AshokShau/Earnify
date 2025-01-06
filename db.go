package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type User struct {
	ID         int64   `bson:"_id,omitempty" json:"_id,omitempty"`
	ReferredBy int64   `bson:"referred_by,omitempty" json:"referred_by,omitempty"`
	ReferredTo []int64 `bson:"referred_to,omitempty" json:"referred_to,omitempty"`
	AccNo      int64   `bson:"acc_no,omitempty" json:"acc_no,omitempty"`
	Balance    float64 `bson:"balance,omitempty" json:"balance,omitempty"`
}

var userColl *mongo.Collection
var ctx = context.TODO()

// Create a new user in the database
func createUser(user User) (*mongo.InsertOneResult, error) {
	result, err := userColl.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Get a user by ID
func getUserByID(id int64) (*User, error) {
	var user User
	filter := bson.M{"_id": id}
	err := userColl.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update a user's balance
func updateUserBalance(id int64, newBalance float64) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"balance": newBalance}}
	result, err := userColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Add a referred user to the user's ReferredTo list without duplication
func addReferredUser(id int64, referredID int64) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	update := bson.M{"$addToSet": bson.M{"referred_to": referredID}}
	result, err := userColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Delete a user by ID
func deleteUserByID(id int64) (*mongo.DeleteResult, error) {
	filter := bson.M{"_id": id}
	result, err := userColl.DeleteOne(ctx, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Update a user's account number
func updateUserAccNo(id int64, newAccNo int64) (*mongo.UpdateResult, error) {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"acc_no": newAccNo}}
	result, err := userColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Check if a user is already referred by another user
func isAlreadyReferred(userID int64) (bool, error) {
	filter := bson.M{"referred_to": userID}
	count, err := userColl.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
