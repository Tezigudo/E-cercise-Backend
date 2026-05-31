package validation

import (
	"fmt"
	"github.com/E-cercise/E-cercise/src/data/request"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func ValidateAddLineEquipment() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var req request.CartItemPostRequest

		// Parse the request body into the struct
		if err := ctx.BodyParser(&req); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid request format",
				"message": err.Error(),
			})
		}

		ctx.Locals("req", req)
		return ctx.Next()
	}
}

func ValidateModifyLineEquipmentRequest() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var req request.CartItemPutRequest

		// Parse the request body into the struct
		if err := ctx.BodyParser(&req); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid request format",
				"message": err.Error(),
			})
		}

		for _, item := range req.Items {
			if item.Quantity <= 0 {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": fmt.Sprintf("Quantity of lineEquipmentID: %v must more than 0", item.LineEquipmentID),
				})
			}
			// Reject non-UUID ids here. Otherwise uuid.MustParse panics in the
			// service, the deferred recover() rolls back and returns nil, and the
			// controller reports HTTP 200 "cart modified successfully" with zero
			// writes — a silent no-op.
			if _, err := uuid.Parse(item.LineEquipmentID); err != nil {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": fmt.Sprintf("Invalid line_equipment_id: %q", item.LineEquipmentID),
				})
			}
		}

		ctx.Locals("req", req)
		return ctx.Next()
	}
}
