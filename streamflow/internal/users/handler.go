package users

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	userService *UserService

	jwtService *JWTService
}

// This is a constructor that injects dependencies
func NewUserHandler(userService *UserService, jwtService *JWTService) *UserHandler {
	return &UserHandler{
		userService: userService,
		jwtService:  jwtService,
	}
}

func (h *UserHandler) CreateUser(c *fiber.Ctx) error {
	var user CreateUserRequest

	if err := c.BodyParser(&user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	//call service to create user
	createdUser, err := h.userService.CreateUser(c.Context(), user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	//generate JWT token
	token, err := h.jwtService.GenerateToken(createdUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User created successfully",
		"token":   token,
		"user":    *createdUser,
	})
}

func (h *UserHandler) LoginUser(c *fiber.Ctx) error {
	var req LoginUserRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	//authenticate user
	user, err := h.userService.AuthenticateUser(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	//generate JWT token for the authenticated user
	token, err := h.jwtService.GenerateToken(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   token,
		"user":    *user,
	})
}

func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	user, err := h.userService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User retrieved successfully",
		"user": *user,
	})
}

// func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	
// }