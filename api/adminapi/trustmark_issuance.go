package adminapi

import (
	"errors"
	"maps"
	"strconv"

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
	storages         model.Backends
	cacheInvalidator TrustMarkCacheInvalidator
}

func (h *trustMarkSpecHandlers) list(c fiber.Ctx) error {
	items, err := h.storages.TrustMarkSpecs.List()
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
	created, err := h.storages.TrustMarkSpecs.Create(&spec)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *trustMarkSpecHandlers) get(c fiber.Ctx) error {
	item, err := h.storages.TrustMarkSpecs.Get(c.Params("trustMarkSpecID"))
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
	updated, err := h.storages.TrustMarkSpecs.Update(specID, &spec)
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
	patched, err := h.storages.TrustMarkSpecs.Patch(specID, updates)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidateTypes(oldType, patched.TrustMarkType)
	return c.JSON(patched)
}

func (h *trustMarkSpecHandlers) delete(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	oldType := h.trustMarkType(specID)
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			// Lock first, so the reads below do not fail on a lock upgrade.
			if err := tx.TrustMarkSpecs.LockSpec(specID); err != nil {
				return err
			}
			if err := recordSpecRevocations(tx, c, specID); err != nil {
				return err
			}
			return tx.TrustMarkSpecs.Delete(specID)
		},
	)
	if err != nil {
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
	spec, err := h.storages.TrustMarkSpecs.Get(specID)
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

// recordSpecRevocations records, in tx, a trust_mark_revoked event for every
// active subject of spec specID, which is about to be deleted.
func recordSpecRevocations(tx *model.Backends, c fiber.Ctx, specID string) error {
	if tx.SubordinateEvents == nil {
		return nil
	}
	spec, err := tx.TrustMarkSpecs.Get(specID)
	if err != nil {
		return err
	}
	active := model.StatusActive
	subjects, err := tx.TrustMarkSpecs.ListSubjects(specID, &active)
	if err != nil {
		return err
	}
	for _, subject := range subjects {
		if err = recordTrustMarkEvent(
			tx, c, model.EventTypeTrustMarkRevoked, spec.TrustMarkType, subject.EntityID, "",
		); err != nil {
			return err
		}
	}
	return nil
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
	storages         model.Backends
	cacheInvalidator TrustMarkCacheInvalidator
	signer           TrustMarkSigner
	// locks serializes the issuance of one trust mark in this process.
	locks *keyedMutex
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
	subjects, err := h.storages.TrustMarkSpecs.ListSubjects(specID, statusFilter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(subjects)
}

// create creates a subject. A subject created active records a
// trust_mark_granted event, whose text a message in the body replaces. With
// "issue": true the subject must be active and its trust mark is signed and
// recorded in the transaction that creates it: if the entity already holds a
// live mark of the type, or the mark cannot be signed, nothing is written.
func (h *trustMarkSubjectHandlers) create(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	var req struct {
		model.AddTrustMarkSubject
		Issue   bool   `json:"issue"`
		Message string `json:"message"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	message, err := parseEventMessage(req.Message)
	if err != nil {
		return writeBadRequest(c, err.Error())
	}
	subject := req.AddTrustMarkSubject
	if subject.EntityID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("entity_id is required"))
	}
	if !subject.Status.Valid() {
		subject.Status = model.StatusActive
	}
	var issuance *pendingIssuance
	if req.Issue {
		if subject.Status != model.StatusActive {
			return writeBadRequest(c, "issue requires an active subject")
		}
		if issuance, err = h.prepareIssuance(specID, subject.EntityID, subject.AdditionalClaims); err != nil {
			return h.handleError(c, err)
		}
		defer issuance.unlock()
	}
	var created *model.TrustMarkSubject
	err = h.storages.InTransaction(
		func(tx *model.Backends) error {
			// CreateSubject reads before it writes: lock the spec first, so
			// concurrent creates under it serialize instead of failing on a
			// lock upgrade.
			if err := tx.TrustMarkSpecs.LockSpec(specID); err != nil {
				return err
			}
			var err error
			if created, err = tx.TrustMarkSpecs.CreateSubject(specID, &subject); err != nil {
				return err
			}
			if created.Status != model.StatusActive {
				return nil
			}
			if err = h.recordSubjectEvent(
				tx, c, specID, model.EventTypeTrustMarkGranted, created.EntityID, message, WithStatus(created.Status),
			); err != nil {
				return err
			}
			if issuance != nil {
				return issuance.record(tx, c, created.ID, message)
			}
			return nil
		},
	)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, created.EntityID)
	issuance.serve(h.signer)
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *trustMarkSubjectHandlers) get(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	subject, err := h.storages.TrustMarkSpecs.GetSubject(specID, subjectID)
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
	var before, updated *model.TrustMarkSubject
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			// Lock first, so the reads below do not fail on a lock upgrade.
			if err := tx.TrustMarkSpecs.LockSubject(specID, subjectID); err != nil {
				return err
			}
			var err error
			if before, err = getTrustMarkSubject(tx.TrustMarkSpecs, specID, subjectID); err != nil {
				return err
			}
			if updated, err = tx.TrustMarkSpecs.UpdateSubject(specID, subjectID, &subject); err != nil {
				return err
			}
			return h.recordSubjectUpdate(tx, c, specID, before, updated)
		},
	)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, before.EntityID, updated.EntityID)
	return c.JSON(updated)
}

func (h *trustMarkSubjectHandlers) delete(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	var before *model.TrustMarkSubject
	err := h.storages.InTransaction(
		func(tx *model.Backends) error {
			// Lock first, so the reads below do not fail on a lock upgrade.
			if err := tx.TrustMarkSpecs.LockSubject(specID, subjectID); err != nil {
				return err
			}
			var err error
			if before, err = getTrustMarkSubject(tx.TrustMarkSpecs, specID, subjectID); err != nil {
				return err
			}
			if err = tx.TrustMarkSpecs.DeleteSubject(specID, subjectID); err != nil {
				return err
			}
			if before.Status != model.StatusActive {
				return nil
			}
			return h.recordSubjectEvent(tx, c, specID, model.EventTypeTrustMarkRevoked, before.EntityID, "")
		},
	)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, before.EntityID)
	return c.SendStatus(fiber.StatusNoContent)
}

// updateStatus changes the status of a subject. An object body may name the
// status the subject must be in; the status then changes only from the status
// that was read, in one conditional UPDATE that is the first statement of the
// transaction. The change, the revocation of the issued instances when the
// subject leaves active, and its event (whose text a message in the body
// replaces) are written in one transaction. With "issue": true the status
// must be active and the trust mark is signed and recorded in that
// transaction too; the change is then made from the status that was read even
// without expected_status.
func (h *trustMarkSubjectHandlers) updateStatus(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")

	req, err := parseStatusRequest(c, trustMarkSubjectStatus)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest(err.Error()))
	}
	if req.Issue && req.Status != model.StatusActive {
		return writeBadRequest(c, "issue requires status active")
	}
	before, err := getTrustMarkSubject(h.storages.TrustMarkSpecs, specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	if !req.Issue && req.ExpectedStatus != nil && *req.ExpectedStatus == req.Status {
		// The status stays the same: nothing to swap, only to check. The
		// result is a snapshot; this writes nothing.
		if before.Status != req.Status {
			return writeConflict(c, errStatusConflict.Error())
		}
		return c.JSON(before)
	}
	var issuance *pendingIssuance
	if req.Issue {
		if req.ExpectedStatus == nil {
			req.ExpectedStatus = &before.Status
		}
		if issuance, err = h.prepareIssuance(specID, before.EntityID, before.AdditionalClaims); err != nil {
			return h.handleError(c, err)
		}
		defer issuance.unlock()
	}
	// The subject is addressed by its DB ID from here on, so a concurrent
	// change of its entity ID cannot make this change hit another subject.
	ref := strconv.FormatUint(uint64(before.ID), 10)
	var updated *model.TrustMarkSubject
	err = h.storages.InTransaction(
		func(tx *model.Backends) error {
			var from model.Status
			var err error
			if from, updated, err = changeSubjectStatus(tx, specID, ref, req); err != nil {
				return err
			}
			if err = h.recordSubjectStatus(tx, c, specID, updated.EntityID, from, req.Status, req.Message); err != nil {
				return err
			}
			if issuance == nil {
				return nil
			}
			if updated.Status != model.StatusActive || updated.EntityID != issuance.entityID {
				return errStatusConflict
			}
			return issuance.record(tx, c, updated.ID, req.Message)
		},
	)
	if err != nil {
		return h.handleError(c, err)
	}
	h.invalidate(specID, before.EntityID)
	issuance.serve(h.signer)
	return c.JSON(updated)
}

// changeSubjectStatus sets, in tx, the status of the subject subjectID to that
// of req and returns the status it changed from and the changed subject. With
// an expected status the change is a compare-and-swap, the first statement of
// the transaction, so it takes the write lock before anything is read and of
// two overlapping changes from the same status exactly one succeeds; the
// other fails with errStatusConflict. When the status is to stay the same the
// swap only takes the lock (its row count is ignored: MySQL reports 0 for a
// same-value UPDATE) and the status is checked on the row read under that
// lock.
func changeSubjectStatus(tx *model.Backends, specID, subjectID string, req *statusRequest) (
	model.Status, *model.TrustMarkSubject, error,
) {
	if req.ExpectedStatus == nil {
		// Lock first, so the reads below do not fail on a lock upgrade.
		if err := tx.TrustMarkSpecs.LockSubject(specID, subjectID); err != nil {
			return 0, nil, err
		}
		current, err := getTrustMarkSubject(tx.TrustMarkSpecs, specID, subjectID)
		if err != nil {
			return 0, nil, err
		}
		updated, err := tx.TrustMarkSpecs.ChangeSubjectStatus(specID, subjectID, req.Status)
		if err != nil {
			return 0, nil, err
		}
		return current.Status, updated, nil
	}
	from := *req.ExpectedStatus
	changed, err := tx.TrustMarkSpecs.TransitionSubjectStatus(specID, subjectID, from, req.Status)
	if err != nil {
		return 0, nil, err
	}
	if from != req.Status && !changed {
		return 0, nil, errStatusConflict
	}
	updated, err := getTrustMarkSubject(tx.TrustMarkSpecs, specID, subjectID)
	if err != nil {
		return 0, nil, err
	}
	if updated.Status != req.Status {
		return 0, nil, errStatusConflict
	}
	return from, updated, nil
}

// getTrustMarkSubject returns the subject subjectID of spec specID, or a
// model.NotFoundError when there is none.
func getTrustMarkSubject(store model.TrustMarkSpecStore, specID, subjectID string) (*model.TrustMarkSubject, error) {
	subject, err := store.GetSubject(specID, subjectID)
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, model.NotFoundError("trust mark subject not found")
	}
	return subject, nil
}

// recordSubjectEvent records, in tx, an event of eventType about the trust
// mark of spec specID held by entityID; see recordTrustMarkEvent.
func (*trustMarkSubjectHandlers) recordSubjectEvent(
	tx *model.Backends, c fiber.Ctx, specID, eventType, entityID, message string, opts ...EventOption,
) error {
	if tx.SubordinateEvents == nil {
		return nil
	}
	spec, err := tx.TrustMarkSpecs.Get(specID)
	if err != nil {
		return err
	}
	return recordTrustMarkEvent(tx, c, eventType, spec.TrustMarkType, entityID, message, opts...)
}

// recordSubjectStatus records, in tx, the event of a subject of spec specID
// changing status; see recordSubjectStatusEvent.
func (*trustMarkSubjectHandlers) recordSubjectStatus(
	tx *model.Backends, c fiber.Ctx, specID, entityID string, from, to model.Status, message string,
) error {
	if tx.SubordinateEvents == nil {
		return nil
	}
	spec, err := tx.TrustMarkSpecs.Get(specID)
	if err != nil {
		return err
	}
	return recordSubjectStatusEvent(tx, c, spec.TrustMarkType, entityID, from, to, message)
}

// recordSubjectUpdate records, in tx, the events of a subject replaced from
// before to updated: a moved entity ID while active revokes the old entity's
// mark and grants the new one's; otherwise the status change is recorded as
// recordSubjectStatusEvent does.
func (h *trustMarkSubjectHandlers) recordSubjectUpdate(
	tx *model.Backends, c fiber.Ctx, specID string, before, updated *model.TrustMarkSubject,
) error {
	if tx.SubordinateEvents == nil {
		return nil
	}
	if before.EntityID == updated.EntityID {
		return h.recordSubjectStatus(tx, c, specID, updated.EntityID, before.Status, updated.Status, "")
	}
	if before.Status == model.StatusActive {
		if err := h.recordSubjectEvent(
			tx, c, specID, model.EventTypeTrustMarkRevoked, before.EntityID, "", WithStatus(updated.Status),
		); err != nil {
			return err
		}
	}
	if updated.Status == model.StatusActive {
		return h.recordSubjectEvent(
			tx, c, specID, model.EventTypeTrustMarkGranted, updated.EntityID, "", WithStatus(updated.Status),
		)
	}
	return nil
}

func (h *trustMarkSubjectHandlers) getAdditionalClaims(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")
	subject, err := h.storages.TrustMarkSpecs.GetSubject(specID, subjectID)
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
	subject, err := h.storages.TrustMarkSpecs.GetSubject(specID, subjectID)
	if err != nil {
		return h.handleError(c, err)
	}
	updatePayload := &model.AddTrustMarkSubject{
		EntityID:         subject.EntityID,
		Status:           subject.Status,
		Description:      subject.Description,
		AdditionalClaims: claims,
	}
	updated, err := h.storages.TrustMarkSpecs.UpdateSubject(specID, subjectID, updatePayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.invalidate(specID, updated.EntityID)
	return c.JSON(updated.AdditionalClaims)
}

func (h *trustMarkSubjectHandlers) copyAdditionalClaims(c fiber.Ctx) error {
	specID := c.Params("trustMarkSpecID")
	subjectID := c.Params("trustMarkSubjectID")

	spec, err := h.storages.TrustMarkSpecs.Get(specID)
	if err != nil {
		return h.handleError(c, err)
	}

	subject, err := h.storages.TrustMarkSpecs.GetSubject(specID, subjectID)
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
	updated, err := h.storages.TrustMarkSpecs.UpdateSubject(specID, subjectID, updatePayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	h.invalidate(specID, updated.EntityID)
	return c.JSON(updated.AdditionalClaims)
}

// invalidate drops the cached trust mark endpoint results of the given
// entities for the spec's trust mark type. It is called after a write
// succeeded, so a failed write invalidates nothing.
func (h *trustMarkSubjectHandlers) invalidate(specID string, entityIDs ...string) {
	if h.cacheInvalidator == nil {
		return
	}
	spec, err := h.storages.TrustMarkSpecs.Get(specID)
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
	if ok, werr := writeIssuanceError(c, err); ok {
		return werr
	}
	if errors.Is(err, errStatusConflict) {
		return writeConflict(c, err.Error())
	}
	if notFound, ok := errors.AsType[model.NotFoundError](err); ok {
		return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(string(notFound)))
	}
	if alreadyExists, ok := errors.AsType[model.AlreadyExistsError](err); ok {
		return c.Status(fiber.StatusConflict).JSON(oidfed.ErrorInvalidRequest(string(alreadyExists)))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
}

// registerTrustMarkIssuance registers TrustMarkSpec and TrustMarkSubject endpoints.
func registerTrustMarkIssuance(
	r fiber.Router, storages model.Backends, cacheInvalidator TrustMarkCacheInvalidator, signer TrustMarkSigner,
) {
	specBase := "/trust-marks/issuance-spec"
	subjectBase := specBase + "/:trustMarkSpecID/subjects"

	specH := &trustMarkSpecHandlers{storages: storages, cacheInvalidator: cacheInvalidator}
	subjectH := &trustMarkSubjectHandlers{
		storages: storages, cacheInvalidator: cacheInvalidator, signer: signer, locks: &trustMarkLocks,
	}

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
