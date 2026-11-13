package adminapi

import (
	"errors"
	"maps"

	oidfed "github.com/go-oidfed/lib"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkCacheInvalidator is implemented by types that cache trust mark
// endpoint results (eligibility check results and issued trust marks) and
// need to be invalidated when trust mark subjects or specs change.
type TrustMarkCacheInvalidator interface {
	// InvalidateTrustMark drops the cached results for one entity and trust mark type.
	InvalidateTrustMark(trustMarkType, entityID string)
	// InvalidateTrustMarkType drops the cached results for all entities of a trust mark type.
	InvalidateTrustMarkType(trustMarkType string)
}

// trustMarkSpecHandlers groups handlers for TrustMarkSpec CRUD endpoints.
type trustMarkSpecHandlers struct {
	store            model.TrustMarkSpecStore
	cacheInvalidator TrustMarkCacheInvalidator
}

func (h *trustMarkSpecHandlers) list(c fiber.Ctx) error {
	items, err := h.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(items)
}

func (h *trustMarkSpecHandlers) create(c fiber.Ctx) error {
	var spec model.AddTrustMarkSpec
	if err := c.Bind().Body(&spec); err != nil {
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

func (h *trustMarkSpecHandlers) get(c fiber.Ctx) error {
	item, err := h.store.Get(c.Params("trustMarkSpecID"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(item)
}

func (h *trustMarkSpecHandlers) update(c fiber.Ctx) error {
	var spec model.AddTrustMarkSpec
	if err := c.Bind().Body(&spec); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if spec.TrustMarkType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("trust_mark_type is required"))
	}
	specID := c.Params("trustMarkSpecID")
	oldType := h.trustMarkType(specID)
	updated, err := h.store.Update(specID, &spec)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidateTypes(oldType, updated.TrustMarkType)
	return c.JSON(updated)
}

func (h *trustMarkSpecHandlers) patch(c fiber.Ctx) error {
	var updates map[string]any
	if err := c.Bind().Body(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	specID := c.Params("trustMarkSpecID")
	oldType := h.trustMarkType(specID)
	patched, err := h.store.Patch(specID, updates)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidateTypes(oldType, patched.TrustMarkType)
	return c.JSON(patched)
}

func (h *trustMarkSpecHandlers) delete(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	oldType := h.trustMarkType(specID)
	if err := h.store.Delete(specID); err != nil {
		return h.handleError(c, err)
	}
	h.invalidateTypes(oldType)
	return c.SendStatus(fiber.StatusNoContent)
}

// trustMarkType returns the trust mark type of the spec before it is changed,
// or "" if there is no cache to invalidate or the spec cannot be read (the
// write that follows then reports the error).
func (h *trustMarkSpecHandlers) trustMarkType(specID string) string {
	if h.cacheInvalidator == nil {
		return ""
	}
	spec, err := h.store.Get(specID)
	if err != nil || spec == nil {
		return ""
	}
	return spec.TrustMarkType
}

// invalidateTypes drops the cached trust mark endpoint results of all
// entities of the given trust mark types. A spec change can affect every
// entity of its type (e.g. a changed eligibility config or lifetime), not only
// the subjects stored in the database.
func (h *trustMarkSpecHandlers) invalidateTypes(trustMarkTypes ...string) {
	if h.cacheInvalidator == nil {
		return
	}
	for _, trustMarkType := range trustMarkTypes {
		if trustMarkType != "" {
			h.cacheInvalidator.InvalidateTrustMarkType(trustMarkType)
		}
	}
}

func (*trustMarkSpecHandlers) handleError(c fiber.Ctx, err error) error {
	if notFound, ok := errors.AsType[model.NotFoundError](err); ok {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
	}
	if alreadyExists, ok := errors.AsType[model.AlreadyExistsError](err); ok {
		return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// trustMarkSubjectHandlers groups handlers for TrustMarkSubject CRUD endpoints.
type trustMarkSubjectHandlers struct {
	store            model.TrustMarkSpecStore
	cacheInvalidator TrustMarkCacheInvalidator
}

func (h *trustMarkSubjectHandlers) list(c fiber.Ctx) error {
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

func (h *trustMarkSubjectHandlers) create(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	var subject model.AddTrustMarkSubject
	if err := c.Bind().Body(&subject); err != nil {
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
	h.invalidate(specID, created.EntityID)
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *trustMarkSubjectHandlers) get(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(subject)
}

func (h *trustMarkSubjectHandlers) update(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	var subject model.AddTrustMarkSubject
	if err := c.Bind().Body(&subject); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if subject.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	oldEntityID := h.entityID(specID, subjectID)
	updated, err := h.store.UpdateSubject(specID, subjectID, &subject)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, oldEntityID, updated.EntityID)
	return c.JSON(updated)
}

func (h *trustMarkSubjectHandlers) delete(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	oldEntityID := h.entityID(specID, subjectID)
	if err := h.store.DeleteSubject(specID, subjectID); err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, oldEntityID)
	return c.SendStatus(fiber.StatusNoContent)
}

// updateStatus changes the status of a subject. An object body may name the
// status the subject must be in; the status then changes only from the status
// that was read. A message in the object body is validated; a subject status
// change records no event that carries it.
func (h *trustMarkSubjectHandlers) updateStatus(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")

	req, err := parseStatusRequest(c, trustMarkSubjectStatus)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if req.ExpectedStatus != nil {
		return h.changeStatusConditionally(c, specID, subjectID, req)
	}
	updated, err := h.store.ChangeSubjectStatus(specID, subjectID, req.Status)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, updated.EntityID)
	return c.JSON(updated)
}

// changeStatusConditionally sets the status of the subject to that of req
// only if the subject is in the status req expects, in one conditional UPDATE,
// and answers 409 if it is not: of two overlapping changes from the same
// status exactly one succeeds. The caches are dropped as soon as the status
// has changed.
func (h *trustMarkSubjectHandlers) changeStatusConditionally(
	c fiber.Ctx, specID, subjectID string, req *statusRequest,
) error {
	before, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	if *req.ExpectedStatus == req.Status {
		// The status stays the same: nothing to swap, only to check. The
		// result is a snapshot; this writes nothing.
		if before.Status != req.Status {
			return writeConflict(c, errStatusConflict.Error())
		}
		return c.JSON(before)
	}
	changed, err := h.store.TransitionSubjectStatus(specID, subjectID, *req.ExpectedStatus, req.Status)
	if err != nil {
		return h.handleError(c, err)
	}
	if !changed {
		return writeConflict(c, errStatusConflict.Error())
	}
	h.invalidate(specID, before.EntityID)
	updated, err := h.store.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(updated)
}

func (h *trustMarkSubjectHandlers) getAdditionalClaims(c fiber.Ctx) error {
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

func (h *trustMarkSubjectHandlers) putAdditionalClaims(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	var claims map[string]any
	if err := c.Bind().Body(&claims); err != nil {
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
	h.invalidate(specID, updated.EntityID)
	return c.JSON(updated.AdditionalClaims)
}

func (h *trustMarkSubjectHandlers) copyAdditionalClaims(c fiber.Ctx) error {
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
	maps.Copy(mergedClaims, spec.AdditionalClaims)
	maps.Copy(mergedClaims, subject.AdditionalClaims)

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
	h.invalidate(specID, updated.EntityID)
	return c.JSON(updated.AdditionalClaims)
}

// entityID returns the entity ID of the subject before it is changed, or ""
// if there is no cache to invalidate or the subject cannot be read (the write
// that follows then reports the error).
func (h *trustMarkSubjectHandlers) entityID(specID, subjectID string) string {
	if h.cacheInvalidator == nil {
		return ""
	}
	subject, err := h.store.GetSubject(specID, subjectID)
	if err != nil || subject == nil {
		return ""
	}
	return subject.EntityID
}

// invalidate drops the cached trust mark endpoint results of the given
// entities for the spec's trust mark type. It is called after a write
// succeeded, so a failed write invalidates nothing.
func (h *trustMarkSubjectHandlers) invalidate(specID string, entityIDs ...string) {
	if h.cacheInvalidator == nil {
		return
	}
	spec, err := h.store.Get(specID)
	if err != nil || spec == nil {
		log.Warn().Err(err).Str("spec", specID).
			Msg("could not invalidate trust mark caches: failed to load trust mark spec")
		return
	}
	for _, entityID := range entityIDs {
		if entityID != "" {
			h.cacheInvalidator.InvalidateTrustMark(spec.TrustMarkType, entityID)
		}
	}
}

func (*trustMarkSubjectHandlers) handleError(c fiber.Ctx, err error) error {
	if notFound, ok := errors.AsType[model.NotFoundError](err); ok {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
	}
	if alreadyExists, ok := errors.AsType[model.AlreadyExistsError](err); ok {
		return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// registerTrustMarkIssuance registers TrustMarkSpec and TrustMarkSubject endpoints.
func registerTrustMarkIssuance(r fiber.Router, store model.TrustMarkSpecStore, cacheInvalidator TrustMarkCacheInvalidator) {
	specBase := "/trust-marks/issuance-spec"
	subjectBase := specBase + "/:trustMarkSpecID/subjects"

	specH := &trustMarkSpecHandlers{store: store, cacheInvalidator: cacheInvalidator}
	subjectH := &trustMarkSubjectHandlers{store: store, cacheInvalidator: cacheInvalidator}

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
