package video

import (
	"fmt"
	"log"
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
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user ID"})
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
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

	// Increment view count when someone starts watching (async to not block streaming)
	go func() {
		if err := h.videoService.IncrementViewCount(c.Context(), videoID); err != nil {
			log.Printf("Failed to increment view count for video %s: %v", videoID.Hex(), err)
		}
	}()

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

	// Serve the HLS playlist file from GridFS
	playlistName := fmt.Sprintf("%s/playlist.m3u8", video.ID.Hex())
	fmt.Printf("🔍 [VIDEO] Looking for GridFS file: %s\n", playlistName)
	
	downloadStream, err := h.videoService.DownloadFromGridFS(c.Context(), playlistName)
	if err != nil {
		fmt.Printf("❌ [VIDEO] GridFS download failed for %s: %v\n", playlistName, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Playlist not found"})
	}
	defer downloadStream.Close()

	fmt.Printf("✅ [VIDEO] GridFS file found, serving HLS playlist: %s\n", playlistName)

	// Read the content to debug what we're actually serving
	buffer := make([]byte, 512) // Read first 512 bytes
	n, readErr := downloadStream.Read(buffer)
	if readErr != nil && readErr.Error() != "EOF" {
		fmt.Printf("❌ [VIDEO] Failed to read from GridFS stream: %v\n", readErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read playlist"})
	}
	
	fmt.Printf("📝 [VIDEO] Playlist content preview (%d bytes): %s\n", n, string(buffer[:n]))
	
	// Reset stream position (create new stream since we can't seek)
	downloadStream.Close()
	downloadStream, err = h.videoService.DownloadFromGridFS(c.Context(), playlistName)
	if err != nil {
		fmt.Printf("❌ [VIDEO] Failed to re-open GridFS stream: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to re-open playlist"})
	}
	defer downloadStream.Close()

	// Alternative approach: Read full content and send directly (more reliable than SendStream)
	// Read all content from GridFS
	fullContent := make([]byte, 0)
	buffer = make([]byte, 1024) // Reuse buffer variable
	
	for {
		n, readErr := downloadStream.Read(buffer)
		if n > 0 {
			fullContent = append(fullContent, buffer[:n]...)
		}
		if readErr != nil {
			if readErr.Error() == "EOF" {
				break
			}
			fmt.Printf("❌ [VIDEO] Error reading GridFS stream: %v\n", readErr)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read playlist"})
		}
	}
	
	fmt.Printf("📊 [VIDEO] Read complete playlist: %d total bytes\n", len(fullContent))
	
	if len(fullContent) == 0 {
		fmt.Printf("❌ [VIDEO] Empty playlist file: %s\n", playlistName)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Empty playlist file"})
	}
	
	// Send the content directly
	c.Set("Content-Length", strconv.Itoa(len(fullContent)))
	err = c.Send(fullContent)
	if err != nil {
		fmt.Printf("❌ [VIDEO] Failed to send content for %s: %v\n", playlistName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to send playlist"})
	}
	
	fmt.Printf("✅ [VIDEO] Successfully sent playlist: %s (%d bytes)\n", playlistName, len(fullContent))
	return nil
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

	// Construct segment filename for GridFS lookup
	segmentFilename := fmt.Sprintf("%s/%s", video.ID.Hex(), segmentName)

	// Set proper headers for video segments
	c.Set("Content-Type", "video/MP2T")
	c.Set("Cache-Control", "public, max-age=3600") // Cache segments for 1 hour
	
	// Add timestamp information to response headers
	c.Set("X-Video-Duration", strconv.FormatFloat(video.Metadata.Duration, 'f', 2, 64))

	// Serve the video segment file from GridFS
	downloadStream, err := h.videoService.DownloadFromGridFS(c.Context(), segmentFilename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Segment not found"})
	}
	defer downloadStream.Close()

	return c.SendStream(downloadStream)
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

	// Serve the thumbnail file from GridFS by its ID
	thumbnailID, err := primitive.ObjectIDFromHex(video.ThumbnailPath)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid thumbnail ID"})
	}

	downloadStream, err := h.videoService.fs.OpenDownloadStream(thumbnailID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Thumbnail not found in GridFS"})
	}
	defer downloadStream.Close()

	return c.SendStream(downloadStream)
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

// GetPopularVideos returns the most viewed videos
func (h *VideoHandler) GetPopularVideos(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit > 50 {
		limit = 50 // Cap at 50 to prevent abuse
	}
	
	videos, err := h.videoService.GetPopularVideos(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get popular videos"})
	}
	
	return c.Status(fiber.StatusOK).JSON(videos)
}

// UpdateVideoStatus manually updates a video's status (for debugging/admin purposes)
func (h *VideoHandler) UpdateVideoStatus(c *fiber.Ctx) error {
	videoID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid video ID"})
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate status
	var status VideoStatus
	switch req.Status {
	case "PENDING":
		status = StatusPending
	case "PROCESSING":
		status = StatusProcessing
	case "COMPLETED":
		status = StatusCompleted
	case "FAILED":
		status = StatusFailed
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid status. Must be PENDING, PROCESSING, COMPLETED, or FAILED"})
	}

	err = h.videoService.UpdateVideoStatus(c.Context(), videoID, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update video status"})
	}

	// Return updated video
	video, err := h.videoService.GetVideoByID(c.Context(), videoID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get updated video"})
	}

	return c.JSON(video)
}

// GetTrendingVideos returns trending videos (recent + high views)
func (h *VideoHandler) GetTrendingVideos(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit > 50 {
		limit = 50 // Cap at 50 to prevent abuse
	}
	
	daysBack, _ := strconv.Atoi(c.Query("days", "7"))
	if daysBack > 30 {
		daysBack = 30 // Cap at 30 days
	}
	
	videos, err := h.videoService.GetTrendingVideos(c.Context(), limit, daysBack)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get trending videos"})
	}
	
	return c.Status(fiber.StatusOK).JSON(videos)
}

// ReprocessVideos manually triggers reprocessing of videos that failed GridFS upload
func (h *VideoHandler) ReprocessVideos(c *fiber.Ctx) error {
	err := h.videoService.ReprocessFailedVideos(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to reprocess videos"})
	}
	
	return c.JSON(fiber.Map{"message": "Video reprocessing completed"})
}

// MigrateVideoFields fixes database field naming inconsistencies
func (h *VideoHandler) MigrateVideoFields(c *fiber.Ctx) error {
	err := h.videoService.MigrateVideoFieldNames(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to migrate video fields"})
	}
	
	return c.JSON(fiber.Map{"message": "Video field migration completed"})
}