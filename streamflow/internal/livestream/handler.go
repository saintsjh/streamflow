package livestream

import (
	"github.com/gofiber/fiber/v2"
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

func (h *LivestreamHandler) StopStream(c* fiber.Ctx) error {
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

func (h *LivestreamHandler) ListStreams(c *fiber.Ctx) error {
	streams, err := h.livestreamService.ListStreams()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list streams",
		})
	}
	return c.JSON(streams)
}