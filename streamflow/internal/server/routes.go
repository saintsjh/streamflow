package server

import (
	"log"
	"streamflow/internal/livestream"
	"streamflow/internal/users"
	"streamflow/internal/video"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func (s *FiberServer) RegisterFiberRoutes() {
	s.App.Get("/", s.HelloWorldHandler)
	s.App.Get("/health", s.healthHandler)

	// User routes (public routes)
	userHandler := users.NewUserHandler(s.userService, s.jwtService)
	s.App.Post("/user/register", userHandler.CreateUser)
	s.App.Post("/user/login", userHandler.LoginUser)

	// Protected routes
	api := s.App.Group("/api", s.authMiddleware)
	api.Get("/user/me", userHandler.GetUser)

	// Video routes
	videoHandler := video.NewVideoHandler(s.videoService)
	api.Post("/video/upload", videoHandler.UploadVideo)
	api.Get("/video/list", videoHandler.ListVideos)
	api.Get("/video/popular", videoHandler.GetPopularVideos)
	api.Get("/video/trending", videoHandler.GetTrendingVideos)
	api.Get("/video/:id", videoHandler.GetVideo)
	api.Put("/video/:id", videoHandler.UpdateVideo)
	api.Patch("/video/:id/status", videoHandler.UpdateVideoStatus)
	api.Delete("/video/:id", videoHandler.DeleteVideo)
	api.Post("/video/reprocess", videoHandler.ReprocessVideos)
	api.Post("/video/migrate", videoHandler.MigrateVideoFields)

	// Public routes (no auth needed)
	s.App.Get("/stream/:id/playlist.m3u8", videoHandler.StreamVideo)
	s.App.Get("/stream/:id/segments/:segment", videoHandler.ServeVideoSegment)
	s.App.Get("/thumbnail/:id", videoHandler.GetVideoThumbnail)
	s.App.Get("/video/:id/timestamp", videoHandler.GetVideoTimestamp)

	// Livestream routes
	livestreamHandler := livestream.NewLivestreamHandler(s.livestreamService)
	api.Post("/livestream/start", livestreamHandler.StartStream)
	api.Post("/livestream/stop", livestreamHandler.StopStream)
	api.Get("/livestream/status/:id", livestreamHandler.GetStreamStatus)
	api.Get("/livestream/streams", livestreamHandler.ListStreams)
	api.Get("/livestream/popular", livestreamHandler.GetPopularStreams)
	api.Get("/livestream/search", livestreamHandler.SearchStreams)

	// WebSocket route for livestreaming
	hub := livestream.NewWebSocketHub()
	go hub.Run()
	streamManager := livestream.NewStreamManager(s.livestreamService)
	webRTCManager, err := livestream.NewWebRTCManager(streamManager)
	if err != nil {
		log.Printf("Failed to create WebRTC manager: %v", err)
		return
	}
	wsHandler := livestream.NewWebSocketHandler(hub, s.livestreamService, webRTCManager)
	
	s.App.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	s.App.Get("/ws", websocket.New(wsHandler.ServeHTTP))
}

func (s *FiberServer) HelloWorldHandler(c *fiber.Ctx) error {
	resp := fiber.Map{
		"message": "Hello World",
	}

	return c.JSON(resp)
}

func (s *FiberServer) healthHandler(c *fiber.Ctx) error {
	return c.JSON(s.db.Health())
}
