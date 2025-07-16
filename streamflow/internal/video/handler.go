package video

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VideoHandler struct {
	videoService *VideoService
}

// constructor
func NewVideoHandler(videoService *VideoService) *VideoHandler {
	return &VideoHandler{videoService: videoService}
}

func (h *VideoHandler) UploadVideo(c *fiber.Ctx) error {
	//get user id from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	title := c.FormValue("title")
	description := c.FormValue("description")

	if title == ""{
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Title is required"})
	}
	
	fileHeader, err := c.FormFile("video")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Video file is required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer file.Close()

	video, err := h.videoService.CreateVideo(c.Context(), file, title, description, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create video"})
	}

	return c.Status(fiber.StatusCreated).JSON(video)
}

func (h *VideoHandler) ListVideos(c *fiber.Ctx) error {
	page,_ := strconv.Atoi(c.Query("page", "1"))
	limit,_ := strconv.Atoi(c.Query("limit", "10"))

	video, err := h.videoService.ListVideos(c.Context(), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list videos"})
	}

	return c.Status(fiber.StatusOK).JSON(video)
}

func (h *VideoHandler) GetVideo(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}

	return c.Status(fiber.StatusOK).JSON(video)
}

func (h *VideoHandler) UpdateVideo(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}
	var req UpdateVideoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	updatedVideo, err := h.videoService.UpdateVideo(c.Context(), videoID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update video"})
	}
	return c.JSON(updatedVideo)
}

func (h *VideoHandler) DeleteVideo(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid video ID",
		})
	}
	if err := h.videoService.DeleteVideo(c.Context(), videoID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete video"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}