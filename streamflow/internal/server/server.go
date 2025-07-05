package server

import (
	"github.com/gofiber/fiber/v2"

	"streamflow/internal/database"
)

type FiberServer struct {
	*fiber.App

	db database.Service
}

func New() *FiberServer {
	server := &FiberServer{
		App: fiber.New(fiber.Config{
			ServerHeader: "streamflow",
			AppName:      "streamflow",
		}),

		db: database.New(),
	}

	return server
}
