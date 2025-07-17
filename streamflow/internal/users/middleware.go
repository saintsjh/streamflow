package users

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// authmiddleware is a middleware that checks if the user is authenticated
// for protected routes
func AuthMiddleware( jwtService *JWTService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":"Unauthorized header required",
			})
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}
		
		//extract token from header if it exists
		token := strings.TrimPrefix(authHeader, "Bearer ")

		//verify token
		claims, err := jwtService.VerifyToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":"Invalid token",
			})
		}

		//set user_id in context for future use
		c.Locals("user_id", claims.UserID)
		c.Locals("userEmail", claims.Email)

		return c.Next()
	}
}