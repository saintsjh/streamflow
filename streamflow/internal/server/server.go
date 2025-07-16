package server

import (
	"github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/cors"
    "github.com/gofiber/fiber/v2/middleware/limiter"
    "time"
    
    "streamflow/internal/config"
    "streamflow/internal/database"
)

type FiberServer struct {
	*fiber.App
	cfg *config.Config
	db database.Service
}

func New() *FiberServer {
	app := fiber.New(fiber.Config{
		ServerHeader: "streamflow",
        AppName:      "streamflow",
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
        IdleTimeout:  cfg.Server.IdleTimeout,
	})

	server := &FiberServer{
		App: app,
		cfg:cfg.
		db: database.New(),
	}
	server.applyMiddleware()

	return server
}

func (s *FiberServer) applyMiddleware(){
	s.App.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(s.cfg.Security.CORSOrigins, ","),
        AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
        AllowHeaders:     "Accept,Authorization,Content-Type",
        AllowCredentials: false,
        MaxAge:           300,
	}))

	s.App.User(limiter.New(limiter.Config{
		Max:        s.cfg.Security.RateLimit,
        Expiration: s.cfg.Security.RateWindow,
        KeyGenerator: func(c *fiber.Ctx) string {
            return c.IP() // limit by IP address
		},
	}))
}
