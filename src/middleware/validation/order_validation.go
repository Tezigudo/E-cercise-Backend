package validation

import (
	"github.com/E-cercise/E-cercise/src/data/request"
	"github.com/gofiber/fiber/v2"
)

func ValidateCheckoutOrder() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var req request.PlaceOrderCartRequest

		if err := ctx.BodyParser(&req); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Invalid request format",
				"message": err.Error(),
			})
		}

		// Reject empty checkouts — otherwise the service commits a real but
		// zero-item, $0 "ghost" order.
		if len(req.LineEquipments) == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "line_equipments must not be empty",
			})
		}

		ctx.Locals("req", req)
		return ctx.Next()
	}
}
