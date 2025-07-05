package video

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
	"streamflow/internal/video"
)

type VideoHandler struct {
	videoService *video.VideoService
}

// constructor
func NewVideoHandler(videoService *video.VideoService) *VideoHandler {
	return &VideoHandler{videoService: videoService}
}

func (h *VideoHandler) UploadVideo(c *fiber.Ctx) error {
	
}

func (h *VideoHandler) ListVideos(c *fiber.Ctx) error {
	
}

func (h *VideoHandler) GetVideo(c *fiber.Ctx) error {

}

func (h *VideoHandler) UpdateVideo(c *fiber.Ctx) error {

}

func (h *VideoHandler) DeleteVideo(c *fiber.Ctx) error {

}