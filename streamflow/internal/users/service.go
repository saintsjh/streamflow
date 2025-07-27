package users

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userCollection *mongo.Collection
}

func NewUserService(db *mongo.Database) *UserService {
	return &UserService{
		userCollection: db.Collection("users"),
	}
}

func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	var existingUser User
	err := s.userCollection.FindOne(ctx, bson.M{"$or": []bson.M{
		{"email": req.Email},
		{"user_name": req.UserName},
	}}).Decode(&existingUser)

	if err == nil {
		return nil, errors.New("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := User{
		ID:        primitive.NewObjectID(),
		Email:     req.Email,
		Password:  string(hashedPassword),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserName:  req.UserName,
	}

	//inject user into database
	_, err = s.userCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UserService) AuthenticateUser(ctx context.Context, email, password string) (*User, error) {
	var user User
	// Find user by email (email is unique)
	err := s.userCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		// Don't specify whether email or password is wrong for security
		return nil, errors.New("invalid credentials")
	}

	// Compare the provided password with the stored hash
	// bcrypt.CompareHashAndPassword handles the comparison securely
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Password doesn't match
		return nil, errors.New("invalid credentials")
	}

	return &user, nil
}

// get user
func (s *UserService) GetUserByID(ctx context.Context, userID primitive.ObjectID) (*User, error) {
	var user User
	err := s.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
