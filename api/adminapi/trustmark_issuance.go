package adminapi

import (
	"errors"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func registerTrustMarkIssuance(r fiber.Router, store model.TrustMarkSpecStore) {
	specBase := "/trust-marks/issuance-spec"
	subjectBase := specBase + "/:trustMarkSpecID/subjects"

	// ===== TrustMarkSpec CRUD =====

	// GET /trust-marks/issuance-spec - List all specs
	r.Get(specBase, func(c *fiber.Ctx) error {
		items, err := store.List()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(items)
	})

	// POST /trust-marks/issuance-spec - Create a spec
	r.Post(specBase, func(c *fiber.Ctx) error {
		var spec model.TrustMarkSpec
		if err := c.BodyParser(&spec); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		if spec.TrustMarkType == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
		}
		created, err := store.Create(&spec)
		if err != nil {
			var alreadyExists model.AlreadyExistsError
			if errors.As(err, &alreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.Status(fiber.StatusCreated).JSON(created)
	})

	// GET /trust-marks/issuance-spec/:trustMarkSpecID - Get a spec
	r.Get(specBase+"/:trustMarkSpecID", func(c *fiber.Ctx) error {
		item, err := store.Get(c.Params("trustMarkSpecID"))
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(item)
	})

	// PUT /trust-marks/issuance-spec/:trustMarkSpecID - Update a spec (full replacement)
	r.Put(specBase+"/:trustMarkSpecID", func(c *fiber.Ctx) error {
		var spec model.TrustMarkSpec
		if err := c.BodyParser(&spec); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		if spec.TrustMarkType == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
		}
		updated, err := store.Update(c.Params("trustMarkSpecID"), &spec)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			var alreadyExists model.AlreadyExistsError
			if errors.As(err, &alreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(updated)
	})

	// PATCH /trust-marks/issuance-spec/:trustMarkSpecID - Partially update a spec
	r.Patch(specBase+"/:trustMarkSpecID", func(c *fiber.Ctx) error {
		var updates map[string]any
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		patched, err := store.Patch(c.Params("trustMarkSpecID"), updates)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			var alreadyExists model.AlreadyExistsError
			if errors.As(err, &alreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(patched)
	})

	// DELETE /trust-marks/issuance-spec/:trustMarkSpecID - Delete a spec
	r.Delete(specBase+"/:trustMarkSpecID", func(c *fiber.Ctx) error {
		if err := store.Delete(c.Params("trustMarkSpecID")); err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// ===== TrustMarkSubject CRUD =====

	// GET /trust-marks/issuance-spec/:trustMarkSpecID/subjects - List subjects
	r.Get(subjectBase, func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		var statusFilter *model.Status
		if statusStr := c.Query("status"); statusStr != "" {
			s, err := model.ParseStatus(statusStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
			}
			statusFilter = &s
		}
		subjects, err := store.ListSubjects(specID, statusFilter)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(subjects)
	})

	// POST /trust-marks/issuance-spec/:trustMarkSpecID/subjects - Create a subject
	r.Post(subjectBase, func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		var subject model.TrustMarkSubject
		if err := c.BodyParser(&subject); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		if subject.EntityID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
		}
		if !subject.Status.Valid() {
			subject.Status = model.StatusActive
		}
		created, err := store.CreateSubject(specID, &subject)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			var alreadyExists model.AlreadyExistsError
			if errors.As(err, &alreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.Status(fiber.StatusCreated).JSON(created)
	})

	// GET /trust-marks/issuance-spec/:trustMarkSpecID/subjects/:trustMarkSubjectID - Get a subject
	r.Get(subjectBase+"/:trustMarkSubjectID", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		subject, err := store.GetSubject(specID, subjectID)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(subject)
	})

	// PUT /trust-marks/issuance-spec/:trustMarkSpecID/subjects/:trustMarkSubjectID - Update a subject
	r.Put(subjectBase+"/:trustMarkSubjectID", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		var subject model.TrustMarkSubject
		if err := c.BodyParser(&subject); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		if subject.EntityID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
		}
		updated, err := store.UpdateSubject(specID, subjectID, &subject)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			var alreadyExists model.AlreadyExistsError
			if errors.As(err, &alreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(updated)
	})

	// DELETE /trust-marks/issuance-spec/:trustMarkSpecID/subjects/:trustMarkSubjectID - Delete a subject
	r.Delete(subjectBase+"/:trustMarkSubjectID", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		if err := store.DeleteSubject(specID, subjectID); err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// PUT /trust-marks/issuance-spec/:trustMarkSpecID/subjects/:trustMarkSubjectID/status - Change status
	r.Put(subjectBase+"/:trustMarkSubjectID/status", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		var req struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		if req.Status == "" {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("status is required"))
		}
		status, err := model.ParseStatus(req.Status)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		updated, err := store.ChangeSubjectStatus(specID, subjectID, status)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(updated)
	})

	// ===== Subject Additional Claims (simplified - just get/set the map) =====

	// GET /.../:trustMarkSubjectID/additional-claims
	r.Get(subjectBase+"/:trustMarkSubjectID/additional-claims", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		subject, err := store.GetSubject(specID, subjectID)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		if subject.AdditionalClaims == nil {
			return c.JSON(map[string]any{})
		}
		return c.JSON(subject.AdditionalClaims)
	})

	// PUT /.../:trustMarkSubjectID/additional-claims
	r.Put(subjectBase+"/:trustMarkSubjectID/additional-claims", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")
		var claims map[string]any
		if err := c.BodyParser(&claims); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		subject, err := store.GetSubject(specID, subjectID)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		subject.AdditionalClaims = claims
		updated, err := store.UpdateSubject(specID, subjectID, subject)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(updated.AdditionalClaims)
	})

	// POST /.../:trustMarkSubjectID/additional-claims - Copy general claims from spec to subject
	// Merges spec claims into subject claims; existing subject claims take precedence on conflict
	r.Post(subjectBase+"/:trustMarkSubjectID/additional-claims", func(c *fiber.Ctx) error {
		specID := c.Params("trustMarkSpecID")
		subjectID := c.Params("trustMarkSubjectID")

		// Get the spec to retrieve general claims
		spec, err := store.Get(specID)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}

		// Get the subject
		subject, err := store.GetSubject(specID, subjectID)
		if err != nil {
			var notFound model.NotFoundError
			if errors.As(err, &notFound) {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
			}
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}

		// Merge spec's additional claims into subject claims
		// Start with spec claims as base, then overlay existing subject claims
		mergedClaims := make(map[string]any)

		// First, copy all spec claims
		for k, v := range spec.AdditionalClaims {
			mergedClaims[k] = v
		}

		// Then, overlay existing subject claims (subject takes precedence)
		for k, v := range subject.AdditionalClaims {
			mergedClaims[k] = v
		}

		subject.AdditionalClaims = mergedClaims

		updated, err := store.UpdateSubject(specID, subjectID, subject)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
		}
		return c.JSON(updated.AdditionalClaims)
	})
}
