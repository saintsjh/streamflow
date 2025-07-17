package server

import (
	"context"
	"streamflow/internal/config"
	"streamflow/internal/database"
	"streamflow/internal/livestream"
	"streamflow/internal/users"
	"streamflow/internal/video"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type FiberServer struct {
	App             *fiber.App
	db              database.Service
	userService     *users.UserService
	jwtService      *users.JWTService
	videoService    *video.VideoService
	livestreamService *livestream.LivestreamService
	cfg             *config.Config
}

func New(cfg *config.Config) *FiberServer {
	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})

	db := database.New()
	userService := users.NewUserService(db.GetDatabase())
	jwtService := users.NewJWTService(cfg.JWT.SecretKey)
	videoService := video.NewVideoService(db.GetDatabase())
	livestreamService := livestream.NewLiveStreamService(db.GetDatabase())

	server := &FiberServer{
		App: app,
		db: db,
		userService: userService,
		jwtService: jwtService,
		videoService: videoService,
		livestreamService: livestreamService,
		cfg: cfg,
	}

	// Apply middleware
	server.applyMiddleware()

	return server
}

func (s *FiberServer) Listen(addr string) error {
	return s.App.Listen(addr)
}

func (s *FiberServer) ShutdownWithContext(ctx context.Context) error {
	return s.App.ShutdownWithContext(ctx)
}

func (s *FiberServer) applyMiddleware(){
	s.App.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(s.cfg.Security.CORSOrigins, ","),
        AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
        AllowHeaders:     "Accept,Authorization,Content-Type",
        AllowCredentials: false,
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
	return s.jwtService.AuthMiddleware()(c)
}

// Custom error handler
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"error": err.Error(),
	})
}
