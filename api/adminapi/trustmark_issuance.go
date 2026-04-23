package adminapi

import (
	"errors"
	"strings"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v2"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// trustMarkSpecHandlers groups handlers for TrustMarkSpec CRUD endpoints.
type trustMarkSpecHandlers struct {
	store model.TrustMarkSpecStore
}

func (h *trustMarkSpecHandlers) list(c *fiber.Ctx) error {
	items, err := h.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(items)
}

func (h *trustMarkSpecHandlers) create(c *fiber.Ctx) error {
	var spec model.AddTrustMarkSpec
	if err := c.BodyParser(&spec); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if spec.TrustMarkType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
	}
	created, err := h.store.Create(&spec)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *trustMarkSpecHandlers) get(c *fiber.Ctx) error {
	item, err := h.store.Get(c.Params("trustMarkSpecID"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(item)
}

func (h *trustMarkSpecHandlers) update(c *fiber.Ctx) error {
	var spec model.AddTrustMarkSpec
	if err := c.BodyParser(&spec); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if spec.TrustMarkType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
	}
	updated, err := h.store.Update(c.Params("trustMarkSpecID"), &spec)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(updated)
}

func (h *trustMarkSpecHandlers) patch(c *fiber.Ctx) error {
	var updates map[string]any
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	patched, err := h.store.Patch(c.Params("trustMarkSpecID"), updates)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(patched)
}

func (h *trustMarkSpecHandlers) delete(c *fiber.Ctx) error {
	if err := h.store.Delete(c.Params("trustMarkSpecID")); err != nil {
		return h.handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (*trustMarkSpecHandlers) handleError(c *fiber.Ctx, err error) error {
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

// trustMarkSubjectHandlers groups handlers for TrustMarkSubject CRUD endpoints.
type trustMarkSubjectHandlers struct {
	store model.TrustMarkSpecStore
}

func (h *trustMarkSubjectHandlers) list(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	var statusFilter *model.Status
	if statusStr := c.Query("status"); statusStr != "" {
		s, err := model.ParseStatus(statusStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
		}
		statusFilter = &s
	}
	subjects, err := h.store.ListSubjects(specID, statusFilter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(subjects)
}

func (h *trustMarkSubjectHandlers) create(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	var subject model.AddTrustMarkSubject
	if err := c.BodyParser(&subject); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if subject.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	if !subject.Status.Valid() {
		subject.Status = model.StatusActive
	}
	created, err := h.store.CreateSubject(specID, &subject)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *trustMarkSubjectHandlers) get(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(subject)
}

func (h *trustMarkSubjectHandlers) update(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	var subject model.AddTrustMarkSubject
	if err := c.BodyParser(&subject); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if subject.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	updated, err := h.store.UpdateSubject(specID, subjectID, &subject)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(updated)
}

func (h *trustMarkSubjectHandlers) delete(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	if err := h.store.DeleteSubject(specID, subjectID); err != nil {
		return h.handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *trustMarkSubjectHandlers) updateStatus(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")

	statusStr := strings.TrimSpace(string(c.Body()))
	if statusStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("status is required"))
	}
	status, err := model.ParseStatus(statusStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	updated, err := h.store.ChangeSubjectStatus(specID, subjectID, status)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(updated)
}

func (h *trustMarkSubjectHandlers) getAdditionalClaims(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	if subject.AdditionalClaims == nil {
		return c.JSON(map[string]any{})
	}
	return c.JSON(subject.AdditionalClaims)
}

func (h *trustMarkSubjectHandlers) putAdditionalClaims(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	var claims map[string]any
	if err := c.BodyParser(&claims); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	updatePayload := &model.AddTrustMarkSubject{
		EntityID:         subject.EntityID,
		Status:           subject.Status,
		Description:      subject.Description,
		AdditionalClaims: claims,
	}
	updated, err := h.store.UpdateSubject(specID, subjectID, updatePayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(updated.AdditionalClaims)
}

func (h *trustMarkSubjectHandlers) copyAdditionalClaims(c *fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")

	spec, err := h.store.Get(specID)
	if err != nil {
		return h.handleError(c, err)
	}

	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}

	// Merge spec's additional claims into subject claims
	// Start with spec claims as base, then overlay existing subject claims
	mergedClaims := make(map[string]any)
	for k, v := range spec.AdditionalClaims {
		mergedClaims[k] = v
	}
	for k, v := range subject.AdditionalClaims {
		mergedClaims[k] = v
	}

	subject.AdditionalClaims = mergedClaims
	updatePayload := &model.AddTrustMarkSubject{
		EntityID:         subject.EntityID,
		Status:           subject.Status,
		Description:      subject.Description,
		AdditionalClaims: mergedClaims,
	}
	updated, err := h.store.UpdateSubject(specID, subjectID, updatePayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(updated.AdditionalClaims)
}

func (*trustMarkSubjectHandlers) handleError(c *fiber.Ctx, err error) error {
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

// registerTrustMarkIssuance registers TrustMarkSpec and TrustMarkSubject endpoints.
func registerTrustMarkIssuance(r fiber.Router, store model.TrustMarkSpecStore) {
	specBase := "/trust-marks/issuance-spec"
	subjectBase := specBase + "/:trustMarkSpecID/subjects"

	specH := &trustMarkSpecHandlers{store: store}
	subjectH := &trustMarkSubjectHandlers{store: store}

	// TrustMarkSpec CRUD
	r.Get(specBase, specH.list)
	r.Post(specBase, specH.create)
	r.Get(specBase+"/:trustMarkSpecID", specH.get)
	r.Put(specBase+"/:trustMarkSpecID", specH.update)
	r.Patch(specBase+"/:trustMarkSpecID", specH.patch)
	r.Delete(specBase+"/:trustMarkSpecID", specH.delete)

	// TrustMarkSubject CRUD
	r.Get(subjectBase, subjectH.list)
	r.Post(subjectBase, subjectH.create)
	r.Get(subjectBase+"/:trustMarkSubjectID", subjectH.get)
	r.Put(subjectBase+"/:trustMarkSubjectID", subjectH.update)
	r.Delete(subjectBase+"/:trustMarkSubjectID", subjectH.delete)
	r.Put(subjectBase+"/:trustMarkSubjectID/status", subjectH.updateStatus)

	// Subject additional claims
	r.Get(subjectBase+"/:trustMarkSubjectID/additional-claims", subjectH.getAdditionalClaims)
	r.Put(subjectBase+"/:trustMarkSubjectID/additional-claims", subjectH.putAdditionalClaims)
	r.Post(subjectBase+"/:trustMarkSubjectID/additional-claims", subjectH.copyAdditionalClaims)
}
