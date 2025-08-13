package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userCollection *mongo.Collection
	validator      *validator.Validate
}

func NewUserService(db *mongo.Database) *UserService {
	service := &UserService{
		userCollection: db.Collection("users"),
		validator:      validator.New(),
	}
	
	// Create unique indexes for email and username to handle race conditions
	service.createIndexes()
	
	return service
}

func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	// Validate request
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}

	// Additional validation for empty email (as expected by tests)
	if strings.TrimSpace(req.Email) == "" {
		return nil, errors.New("email is required")
	}

	// Normalize email to lowercase for consistency
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.UserName = strings.TrimSpace(req.UserName)

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

	// Use InsertOne which will fail if unique constraints are violated
	// This handles race conditions better than FindOne + InsertOne
	_, err = s.userCollection.InsertOne(ctx, user)
	if err != nil {
		// Check if it's a duplicate key error
		if mongo.IsDuplicateKeyError(err) {
			return nil, errors.New("user already exists")
		}
		return nil, err
	}

	return &user, nil
}

func (s *UserService) AuthenticateUser(ctx context.Context, email, password string) (*User, error) {
	// Normalize email to match creation logic
	email = strings.ToLower(strings.TrimSpace(email))
	
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

// createIndexes creates unique indexes for email and username to prevent duplicates
func (s *UserService) createIndexes() {
	ctx := context.Background()
	
	// Create unique index for email
	emailIndex := mongo.IndexModel{
		Keys:    bson.D{{"email", 1}},
		Options: options.Index().SetUnique(true),
	}
	
	// Create unique index for username
	usernameIndex := mongo.IndexModel{
		Keys:    bson.D{{"user_name", 1}},
		Options: options.Index().SetUnique(true),
	}
	
	// Create the indexes (ignore errors as they might already exist)
	s.userCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{emailIndex, usernameIndex})
}
