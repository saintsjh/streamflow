package livestream

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LivestreamHandler struct {
	livestreamService *LivestreamService
}

func NewLivestreamHandler(livestreamService *LivestreamService) *LivestreamHandler {
	return &LivestreamHandler{livestreamService: livestreamService}
}

func (h *LivestreamHandler) StartStream(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	var req StartStreamRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	stream, err := h.livestreamService.StartStream(userID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start stream",
		})
	}

	return c.Status(fiber.StatusOK).JSON(stream)
}

func (h *LivestreamHandler) StopStream(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	streamID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid stream ID",
		})
	}
	_, err = h.livestreamService.StopStream(userID, streamID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to stop stream",
		})
	}
	return c.SendStatus(fiber.StatusNoContent)

}

func (h *LivestreamHandler) GetStreamStatus(c *fiber.Ctx) error {
	streamID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid stream ID",
		})
	}

	status, err := h.livestreamService.GetStreamStatus(streamID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get stream status",
		})
	}

	return c.JSON(status)
}

// ListStreams handles requests to list all currently live streams.
func (h *LivestreamHandler) ListStreams(c *fiber.Ctx) error {
	streams, err := h.livestreamService.ListStreams()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not fetch streams"})
	}
	return c.Status(fiber.StatusOK).JSON(streams)
}

// GetStream handles requests for a single stream's details.
func (h *LivestreamHandler) GetStream(c *fiber.Ctx) error {
	streamID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stream ID"})
	}

	stream, err := h.livestreamService.GetStreamStatus(streamID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "stream not found"})
	}
	return c.Status(fiber.StatusOK).JSON(stream)
}

// SearchStreams handles requests to search for live streams.
func (h *LivestreamHandler) SearchStreams(c *fiber.Ctx) error {
	query := c.Query("q")
	streams, err := h.livestreamService.SearchStreams(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not perform search"})
	}
	return c.Status(fiber.StatusOK).JSON(streams)
}

// GetPopularStreams handles requests to get streams ordered by viewer count
func (h *LivestreamHandler) GetPopularStreams(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit > 50 {
		limit = 50 // Cap at 50 to prevent abuse  
	}
	
	streams, err := h.livestreamService.GetPopularStreams(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not fetch popular streams"})
	}
	return c.Status(fiber.StatusOK).JSON(streams)
}

// HandleWebSocket is the handler for upgrading connections to WebSocket.
func (h *LivestreamHandler) HandleWebSocket(c *fiber.Ctx) error {
	// Let the fiber middleware handle the upgrade.
	// The actual connection logic is in websocket.go's ServeHTTP method.
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}
