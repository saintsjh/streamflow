package video

import (
	"path/filepath"
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

	// Validate the uploaded file
	if err := ValidateVideoFile(fileHeader); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
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

// StreamVideo serves the HLS playlist for video streaming with seeking support
func (h *VideoHandler) StreamVideo(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}

	if video.Status != StatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Video is not ready for streaming"})
	}

	if video.HLSPath == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video stream not available"})
	}

	// Get seek time from query parameter (in seconds)
	seekTimeStr := c.Query("t", "")
	var seekTime float64
	if seekTimeStr != "" {
		seekTime, err = strconv.ParseFloat(seekTimeStr, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid seek time format"})
		}
		if seekTime < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Seek time cannot be negative"})
		}
		if seekTime > video.Metadata.Duration {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Seek time exceeds video duration"})
		}
	}

	// Set proper headers for HLS streaming
	c.Set("Content-Type", "application/vnd.apple.mpegurl")
	c.Set("Cache-Control", "public, max-age=10")
	
	// Add seeking information to response headers
	if seekTime > 0 {
		c.Set("X-Seek-Time", strconv.FormatFloat(seekTime, 'f', 2, 64))
		c.Set("X-Video-Duration", strconv.FormatFloat(video.Metadata.Duration, 'f', 2, 64))
	}
	
	// Serve the HLS playlist file
	return c.SendFile(video.HLSPath)
}

// ServeVideoSegment serves individual video segments for HLS streaming with timestamp support
func (h *VideoHandler) ServeVideoSegment(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	segmentName := c.Params("segment")
	if segmentName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Segment name required"})
	}

	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}

	if video.Status != StatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Video is not ready for streaming"})
	}

	// Construct segment path
	segmentPath := filepath.Join(filepath.Dir(video.HLSPath), segmentName)
	
	// Set proper headers for video segments
	c.Set("Content-Type", "video/MP2T")
	c.Set("Cache-Control", "public, max-age=3600") // Cache segments for 1 hour
	
	// Add timestamp information to response headers
	c.Set("X-Video-Duration", strconv.FormatFloat(video.Metadata.Duration, 'f', 2, 64))
	
	// Serve the video segment file
	return c.SendFile(segmentPath)
}

// GetVideoThumbnail serves the video thumbnail
func (h *VideoHandler) GetVideoThumbnail(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}

	if video.ThumbnailPath == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Thumbnail not available"})
	}

	// Set proper headers for image
	c.Set("Content-Type", "image/jpeg")
	c.Set("Cache-Control", "public, max-age=86400") // Cache thumbnails for 24 hours
	
	// Serve the thumbnail file
	return c.SendFile(video.ThumbnailPath)
}

// GetVideoTimestamp returns the current timestamp and duration information
func (h *VideoHandler) GetVideoTimestamp(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}

	if video.Status != StatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Video is not ready for streaming"})
	}

	// Get current time from query parameter (in seconds)
	currentTimeStr := c.Query("current", "0")
	currentTime, err := strconv.ParseFloat(currentTimeStr, 64)
	if err != nil {
		currentTime = 0
	}

	// Ensure current time doesn't exceed video duration
	if currentTime > video.Metadata.Duration {
		currentTime = video.Metadata.Duration
	}

	return c.JSON(fiber.Map{
		"video_id": video.ID.Hex(),
		"current_time": currentTime,
		"duration": video.Metadata.Duration,
		"remaining": video.Metadata.Duration - currentTime,
		"progress_percentage": (currentTime / video.Metadata.Duration) * 100,
	})
}