package adminapi

import (
	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// registerUsers wires handlers using a UsersStore abstraction.
func registerUsers(r fiber.Router, users model.UsersStore) {
	g := r.Group("/users")

	g.Get("/", func(c *fiber.Ctx) error {
		list, err := users.List()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(list)
	})

	type createReq struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	g.Post("/", func(c *fiber.Ctx) error {
		var req createReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
		}
		if req.Username == "" || req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("username and password are required"))
		}
		u, err := users.Create(req.Username, req.Password, req.DisplayName)
		if err != nil {
			if _, ok := err.(model.AlreadyExistsError); ok {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest("user already exists"))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.Status(fiber.StatusCreated).JSON(u)
	})

	type updateReq struct {
		DisplayName *string `json:"display_name"`
		Password    *string `json:"password"`
		Disabled    *bool   `json:"disabled"`
	}
	g.Put("/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		var req updateReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
		}
		u, err := users.Update(username, req.DisplayName, req.Password, req.Disabled)
		if err != nil {
			if _, ok := err.(model.NotFoundError); ok {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("user not found"))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(u)
	})

	g.Get("/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		u, err := users.Get(username)
		if err != nil {
			if _, ok := err.(model.NotFoundError); ok {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("user not found"))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(u)
	})

	g.Delete("/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		if err := users.Delete(username); err != nil {
			if _, ok := err.(model.NotFoundError); ok {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("user not found"))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
