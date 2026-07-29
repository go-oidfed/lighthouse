package lighthouse

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

func handleError(ctx *fiber.Ctx, err error) error {
	// Status code defaults to 500
	code := fiber.StatusInternalServerError
	msg := err.Error()

	if e, ok := errors.AsType[*fiber.Error](err); ok {
		code = e.Code
		msg = e.Error()
	}
	return ctx.Status(code).JSON(fiber.Map{"error": msg})
}
