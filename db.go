package main

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User represents the structure of a user document in MongoDB
type User struct {
	ID            int64   `bson:"_id,omitempty" json:"_id,omitempty"`
	Referrer      int64   `bson:"referrer,omitempty" json:"referrer,omitempty"`
	ReferredUsers []int64 `bson:"referred_users,omitempty" json:"referred_users,omitempty"`
	AccNo         int64   `bson:"acc_no,omitempty" json:"acc_no,omitempty"`
	Balance       float64 `bson:"balance,omitempty" json:"balance,omitempty"`
}

var userColl *mongo.Collection
var ctx = context.TODO()

// Add a new user to the database
func addUser(user User) error {
	// Ensure the user is unique by checking both _id and acc_no
	filter := bson.M{"$or": []bson.M{
		{"_id": user.ID},
	}}

	count, err := userColl.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("user with ID %d already exists", user.ID)
	}

	_, err = userColl.InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to add user: %v", err)
	}

	return nil
}

// Update a user's referrer and the referrer's referred users
func referUser(referrerID, newUserID int64) error {
	// Check if referrer exists
	referrer := User{}
	err := userColl.FindOne(ctx, bson.M{"_id": referrerID}).Decode(&referrer)
	if err != nil {
		return fmt.Errorf("referrer with ID %d does not exist", referrerID)
	}

	// Add the new user
	newUser := User{
		ID:       newUserID,
		Referrer: referrerID,
		Balance:  0,
	}

	err = addUser(newUser)
	if err != nil {
		return err
	}

	// Update the referrer to add the new user to their referred_users
	_, err = userColl.UpdateOne(ctx, bson.M{"_id": referrerID}, bson.M{"$push": bson.M{"referred_users": newUserID}})
	if err != nil {
		return fmt.Errorf("failed to update referrer's referred users: %v", err)
	}
	return nil
}

// Retrieve a user's details
func getUser(userID int64) (*User, error) {
	user := User{}
	err := userColl.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// // Retrieve all users referred by a specific user
// func getReferredUsers(referrerID int64) ([]User, error) {
// 	// Fetch the referrer's referred_users list
// 	referrer, err := getUser(referrerID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Fetch details of all referred users
// 	filter := bson.M{"_id": bson.M{"$in": referrer.ReferredUsers}}
// 	cursor, err := userColl.Find(ctx, filter)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to retrieve referred users: %v", err)
// 	}
// 	defer cursor.Close(ctx)

// 	var referredUsers []User
// 	if err = cursor.All(ctx, &referredUsers); err != nil {
// 		return nil, fmt.Errorf("failed to decode referred users: %v", err)
// 	}
// 	return referredUsers, nil
// }

// Update a user's balance
func updateUserBalance(userID int64, amount float64) error {
	_, err := userColl.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$inc": bson.M{"balance": amount}})
	if err != nil {
		return fmt.Errorf("failed to update balance for user %d: %v", userID, err)
	}
	return nil
}

func removeBalance(userID int64, amount float64) (float64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount to remove must be greater than zero")
	}
	filter := bson.M{"_id": userID}
	update := bson.M{"$inc": bson.M{"balance": -amount}}
	options := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updatedUser User
	err := userColl.FindOneAndUpdate(ctx, filter, update, options).Decode(&updatedUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, fmt.Errorf("user with ID %d does not exist", userID)
		}
		return 0, fmt.Errorf("failed to update balance: %v", err)
	}

	if updatedUser.Balance < 0 {
		_, rollbackErr := userColl.UpdateOne(ctx, filter, bson.M{"$inc": bson.M{"balance": amount}})
		if rollbackErr != nil {
			return 0, fmt.Errorf("balance went negative, rollback failed: %v", rollbackErr)
		}
		return 0, fmt.Errorf("insufficient balance for user %d", userID)
	}

	return updatedUser.Balance, nil
}

// Update a user's account number
func updateUserAccNo(userID int64, accNo int64) error {
	_, err := userColl.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"acc_no": accNo}})
	if err != nil {
		return fmt.Errorf("failed to update acc_no for user %d: %v", userID, err)
	}
	return nil
}

func getAllUsers() ([]User, error) {
	cursor, err := userColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %v", err)
	}
	defer cursor.Close(ctx)

	var users []User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %v", err)
	}
	return users, nil
}
