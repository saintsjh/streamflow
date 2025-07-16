package users

import(
	"strings"
    "github.com/gofiber/fiber/v2"
    "streamflow/internal/users"
)


// authmiddleware is a middleware that checks if the user is authenticated
// for protected routes
func AuthMiddleware( jwtService *users.JWTService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":"Unauthorized header required",
			})
		}

		//extract token from header if it exists
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":"Invalid token format",
			})
		}

		//verify token
		claims, err := jwtService.VerifyToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":"Invalid token",
			})
		}

		//set user_id in context for future use
		c.Locals("user_id", claims.UserID)
		c.locals("userEmail", claims.Email)

		return c.Next()
	}
}