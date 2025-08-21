package server

import (
	"context"
	"fmt"
	"log"
	"streamflow/internal/config"
	"streamflow/internal/database"
	"streamflow/internal/livestream"
	"streamflow/internal/users"
	"streamflow/internal/video"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type FiberServer struct {
	App               *fiber.App
	db                database.Service
	userService       *users.UserService
	jwtService        *users.JWTService
	videoService      *video.VideoService
	livestreamService *livestream.LivestreamService
	cfg               *config.Config
	maxFileSize       int64 // Store for error messages
}

func New(cfg *config.Config) *FiberServer {
	// Add some buffer to the configured max file size for form data overhead (video + thumbnail + form fields)
	bodyLimit := cfg.Video.MaxFileSize + (10 * 1024 * 1024) // Add 10MB buffer for form data overhead
	
	server := &FiberServer{
		cfg:         cfg,
		maxFileSize: cfg.Video.MaxFileSize,
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: server.customErrorHandler, // Use method instead of standalone function
		BodyLimit:    int(bodyLimit), // Use configured max file size + buffer
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	})

	db := database.New()
	userService := users.NewUserService(db.GetDatabase())
	jwtService := users.NewJWTService(cfg.JWT.SecretKey)
	videoService := video.NewVideoService(db.GetDatabase())
	livestreamService := livestream.NewLiveStreamService(db.GetDatabase())

	// Complete the server initialization
	server.App = app
	server.db = db
	server.userService = userService
	server.jwtService = jwtService
	server.videoService = videoService
	server.livestreamService = livestreamService

	// Apply middleware
	server.applyMiddleware()

	return server
}

func (s *FiberServer) Listen(addr string) error {
	return s.App.Listen(addr)
}

func (s *FiberServer) ShutdownWithContext(ctx context.Context) error {
	// Close database connection first
	if err := s.db.Close(); err != nil {
		log.Printf("Error closing database connection: %v", err)
	} else {
		log.Println("Database connection closed successfully")
	}

	// Then shutdown the Fiber app
	return s.App.ShutdownWithContext(ctx)
}

func (s *FiberServer) applyMiddleware() {
	s.App.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return true // Allow all origins for development
		},
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Accept,Authorization,Content-Type,X-CSRF-Token",
		AllowCredentials: true,
		MaxAge:           300,
	}))

	s.App.Use(limiter.New(limiter.Config{
		Max:        s.cfg.Security.RateLimit,
		Expiration: s.cfg.Security.RateWindow,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() // limit by IP address
		},
	}))
}

// AuthMiddleware returns the authentication middleware
func (s *FiberServer) authMiddleware(c *fiber.Ctx) error {
	err := s.jwtService.Middleware()(c)
	if err != nil {
		log.Printf("Authentication failed for %s %s: %v", c.Method(), c.Path(), err)
		return err
	}
	return nil
}

// Custom error handler (now a method of FiberServer)
func (s *FiberServer) customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// Log important errors only
	if code >= 500 || code == fiber.StatusRequestEntityTooLarge {
		log.Printf("Error %d on %s %s: %v", code, c.Method(), c.Path(), err)
	}

	// Provide more helpful error messages for common issues
	errorMsg := err.Error()
	if code == fiber.StatusRequestEntityTooLarge {
		maxSizeMB := s.maxFileSize / (1024 * 1024)
		errorMsg = fmt.Sprintf("File too large. Maximum allowed size is %dMB for video uploads.", maxSizeMB)
	}

	return c.Status(code).JSON(fiber.Map{
		"error": errorMsg,
	})
}
