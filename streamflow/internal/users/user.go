package users

import (
	"time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID primitive.ObjectID `bson:"_id"`
	Email string `bson:"email"`
	Password string `bson:"password"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	UserName string `bson:"user_name"`
}

type CreateUserRequest struct {
	UserName string `json:"user_name" validate:"required,min=3,max=32"`
	Email string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`

}

type AuthResponse struct {
	Token string `json:"token"`
	User User `json:"user"`
}