package adminapi

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// TrustMarkSigner signs trust marks for the admin API's immediate issuance.
type TrustMarkSigner interface {
	// SignTrustMark signs a trust mark of trustMarkType for sub with the given
	// additional claims and a new jti, as the trust mark endpoint would but
	// without eligibility checks, and without recording or caching it. It
	// returns ErrIssuanceUnavailable if this entity does not issue trust
	// marks of the type.
	SignTrustMark(trustMarkType, sub string, claims map[string]any) (SignedTrustMark, error)
	// ServeTrustMark makes the trust mark endpoint serve signed, which the
	// admin API has recorded, to the next requests for trustMarkType and sub:
	// it puts it into the endpoint's issued trust mark cache for cacheTTL,
	// bounded by its expiry, as the endpoint caches the marks it signs. A
	// cacheTTL of 0 caches nothing, like the endpoint.
	ServeTrustMark(trustMarkType, sub string, signed SignedTrustMark, cacheTTL time.Duration)
}

// SignedTrustMark is a trust mark TrustMarkSigner signed.
type SignedTrustMark struct {
	// JWT is the signed trust mark.
	JWT string
	// JTI is its JWT ID, the key of the issued instance.
	JTI string
	// ExpiresAt is the unix time it expires; 0 if it does not.
	ExpiresAt int64
}

var (
	// ErrIssuanceUnavailable means this entity cannot issue the trust mark:
	// no signer is configured or the trust mark type is not one it issues.
	ErrIssuanceUnavailable = errors.New("trust mark issuance is not configured for this trust mark type")

	errAlreadyHeld   = errors.New("the entity already holds an active trust mark of this type")
	errSubjectActive = errors.New(
		"the entity is an active subject of this trust mark type; change the subject's status instead",
	)
)

// trustMarkLocks serializes, within this process, the admin API's issuance
// and revocation of the trust mark of one type held by one entity, so the
// check that no live mark is held and the write that follows cannot
// interleave with another issuance or revocation of the same mark.
var trustMarkLocks keyedMutex

// keyedMutex is a set of mutexes addressed by key; entries exist only while
// held or awaited. The zero value is ready to use.
type keyedMutex struct {
	mu      sync.Mutex
	entries map[string]*keyedEntry
}

type keyedEntry struct {
	mu    sync.Mutex
	users int // holders and waiters
}

// lock locks the trust mark of trustMarkType held by entityID and returns the
// function that unlocks it.
func (k *keyedMutex) lock(trustMarkType, entityID string) func() {
	key := trustMarkType + "\x00" + entityID
	k.mu.Lock()
	if k.entries == nil {
		k.entries = map[string]*keyedEntry{}
	}
	e := k.entries[key]
	if e == nil {
		e = &keyedEntry{}
		k.entries[key] = e
	}
	e.users++
	k.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		if e.users--; e.users == 0 {
			delete(k.entries, key)
		}
		k.mu.Unlock()
	}
}

// pendingIssuance is a trust mark signed for a subject, to be recorded in the
// transaction that creates or activates the subject.
type pendingIssuance struct {
	trustMarkType string
	entityID      string
	cacheTTL      time.Duration
	signed        SignedTrustMark
	unlock        func()
}

// serve makes the trust mark endpoint serve the recorded trust mark (see
// TrustMarkSigner.ServeTrustMark), so the entity's next request gets this
// mark rather than a newly signed second instance. It is called after the
// commit and after the caches were dropped; a nil p does nothing.
func (p *pendingIssuance) serve(signer TrustMarkSigner) {
	if p == nil {
		return
	}
	signer.ServeTrustMark(p.trustMarkType, p.entityID, p.signed, p.cacheTTL)
}

// prepareIssuance signs the trust mark of spec specID for entityID, under the
// lock of the (type, entity) pair, which the caller releases with unlock once
// the issuance is committed or abandoned. The claims are subjectClaims if not
// empty, else the spec's additional claims, as for the trust mark endpoint.
// It fails with errAlreadyHeld if the entity holds a live mark of the type,
// and with ErrIssuanceUnavailable if no mark can be issued.
func (h *trustMarkSubjectHandlers) prepareIssuance(
	specID, entityID string, subjectClaims map[string]any,
) (*pendingIssuance, error) {
	if h.signer == nil || h.storages.TrustMarkInstances == nil {
		return nil, ErrIssuanceUnavailable
	}
	spec, err := h.storages.TrustMarkSpecs.Get(specID)
	if err != nil {
		return nil, err
	}
	unlock := h.locks.lock(spec.TrustMarkType, entityID)
	held, err := h.storages.TrustMarkInstances.HasActiveInstance(spec.TrustMarkType, entityID)
	if err == nil && held {
		err = errAlreadyHeld
	}
	if err != nil {
		unlock()
		return nil, err
	}
	claims := subjectClaims
	if len(claims) == 0 {
		claims = spec.AdditionalClaims
	}
	signed, err := h.signer.SignTrustMark(spec.TrustMarkType, entityID, claims)
	if err != nil {
		unlock()
		return nil, err
	}
	return &pendingIssuance{
		trustMarkType: spec.TrustMarkType, entityID: entityID, signed: signed, unlock: unlock,
		cacheTTL: time.Duration(spec.CacheTTL) * time.Second,
	}, nil
}

// record records, in tx, the issued instance of the trust mark for the
// subject subjectID and a trust_mark_issued event. Checking again in tx that
// no live mark is held makes a concurrent issuance by another process fail
// with errAlreadyHeld instead of recording a second live mark.
func (p *pendingIssuance) record(tx *model.Backends, c fiber.Ctx, subjectID uint, message string) error {
	held, err := tx.TrustMarkInstances.HasActiveInstance(p.trustMarkType, p.entityID)
	if err != nil {
		return err
	}
	if held {
		return errAlreadyHeld
	}
	if err = tx.TrustMarkInstances.Create(
		&model.IssuedTrustMarkInstance{
			JTI:                p.signed.JTI,
			ExpiresAt:          int(p.signed.ExpiresAt),
			TrustMarkSubjectID: subjectID,
			TrustMarkType:      p.trustMarkType,
			Subject:            p.entityID,
		},
	); err != nil {
		return err
	}
	return recordTrustMarkEvent(tx, c, model.EventTypeTrustMarkIssued, p.trustMarkType, p.entityID, message)
}

// recordTrustMarkEvent records, in tx, an event of eventType about the trust
// mark of trustMarkType held by entityID. The event is keyed by the entity ID
// and carries the entity's subordinate ID when it is a subordinate, so it
// also shows in that subordinate's history. Its message is message or, when
// that is empty, names the mark by its spec's description (if any) and type.
func recordTrustMarkEvent(
	tx *model.Backends, c fiber.Ctx, eventType, trustMarkType, entityID, message string, opts ...EventOption,
) error {
	if tx.SubordinateEvents == nil {
		return nil
	}
	if message == "" {
		message = trustMarkType
		if tx.TrustMarkSpecs != nil {
			spec, err := tx.TrustMarkSpecs.GetByType(trustMarkType)
			if err == nil && spec.Description != "" {
				message = spec.Description + " (" + trustMarkType + ")"
			} else if _, notFound := errors.AsType[model.NotFoundError](err); err != nil && !notFound {
				return err
			}
		}
	}
	var subordinateID uint
	if tx.Subordinates != nil {
		sub, err := tx.Subordinates.Get(entityID)
		if err != nil {
			return err
		}
		if sub != nil {
			subordinateID = sub.ID
		}
	}
	return RecordEvent(
		tx.SubordinateEvents, subordinateID, eventType,
		append([]EventOption{WithEntityID(entityID), WithMessage(message), WithActor(GetActor(c))}, opts...)...,
	)
}

// recordSubjectStatusEvent records, in tx, the event of a trust mark subject
// of trustMarkType changing from status from to status to: trust_mark_granted
// when it becomes active, trust_mark_revoked when it leaves active, else
// trust_mark_status_updated. Nothing is recorded when the status stays.
func recordSubjectStatusEvent(
	tx *model.Backends, c fiber.Ctx, trustMarkType, entityID string, from, to model.Status, message string,
) error {
	var eventType string
	switch {
	case from == to:
		return nil
	case to == model.StatusActive:
		eventType = model.EventTypeTrustMarkGranted
	case from == model.StatusActive:
		eventType = model.EventTypeTrustMarkRevoked
	default:
		eventType = model.EventTypeTrustMarkStatusUpdated
	}
	return recordTrustMarkEvent(tx, c, eventType, trustMarkType, entityID, message, WithStatus(to))
}

// writeIssuanceError answers the errors specific to issuance and revocation
// with their machine-readable codes, and reports whether err was one.
func writeIssuanceError(c fiber.Ctx, err error) (bool, error) {
	switch {
	case errors.Is(err, errAlreadyHeld):
		return true, writeCodedError(c, fiber.StatusConflict, "already_held", err.Error())
	case errors.Is(err, errSubjectActive):
		return true, writeCodedError(c, fiber.StatusConflict, "subject_active", err.Error())
	case errors.Is(err, ErrIssuanceUnavailable):
		return true, writeCodedError(c, fiber.StatusServiceUnavailable, "issuance_unavailable", err.Error())
	}
	return false, nil
}

const (
	defaultInstanceLimit = 50
	maxInstanceLimit     = 100
)

// trustMarkInstanceHandlers groups the handlers for issued trust mark instances.
type trustMarkInstanceHandlers struct {
	storages         model.Backends
	cacheInvalidator TrustMarkCacheInvalidator
	locks            *keyedMutex
}

type trustMarkInstanceResponse struct {
	JTI           string `json:"jti"`
	TrustMarkType string `json:"trust_mark_type"`
	Subject       string `json:"subject"`
	IssuedAt      int    `json:"issued_at"`
	ExpiresAt     int    `json:"expires_at"`
	State         string `json:"state"`
}

func (h *trustMarkInstanceHandlers) list(c fiber.Ctx) error {
	q := model.InstanceQuery{
		TrustMarkType: c.Query("trust_mark_type"),
		Subject:       c.Query("sub"),
		Limit:         defaultInstanceLimit,
		Now:           time.Now().Unix(),
	}
	switch state := model.TrustMarkInstanceStatus(c.Query("state")); state {
	case "":
	case model.TrustMarkStatusActive, model.TrustMarkStatusRevoked, model.TrustMarkStatusExpired:
		q.State = state
	default:
		return writeBadRequest(c, "state must be one of active, revoked, expired")
	}
	if s := c.Query("limit"); s != "" {
		limit, err := strconv.Atoi(s)
		if err != nil || limit <= 0 {
			return writeBadRequest(c, "invalid limit parameter")
		}
		q.Limit = min(limit, maxInstanceLimit)
	}
	if s := c.Query("offset"); s != "" {
		offset, err := strconv.Atoi(s)
		if err != nil || offset < 0 {
			return writeBadRequest(c, "invalid offset parameter")
		}
		q.Offset = offset
	}
	instances, total, err := h.storages.TrustMarkInstances.List(q)
	if err != nil {
		return writeServerError(c, err)
	}
	out := make([]trustMarkInstanceResponse, len(instances))
	for i, inst := range instances {
		out[i] = trustMarkInstanceResponse{
			JTI:           inst.JTI,
			TrustMarkType: inst.TrustMarkType,
			Subject:       inst.Subject,
			IssuedAt:      inst.CreatedAt,
			ExpiresAt:     inst.ExpiresAt,
			State:         string(instanceState(inst, q.Now)),
		}
	}
	return c.JSON(
		fiber.Map{
			"instances":  out,
			"pagination": fiber.Map{"total": total, "limit": q.Limit, "offset": q.Offset},
		},
	)
}

// instanceState derives the state of inst at the unix time now.
func instanceState(inst model.IssuedTrustMarkInstance, now int64) model.TrustMarkInstanceStatus {
	switch {
	case inst.Revoked:
		return model.TrustMarkStatusRevoked
	case inst.ExpiresAt != 0 && int64(inst.ExpiresAt) <= now:
		return model.TrustMarkStatusExpired
	}
	return model.TrustMarkStatusActive
}

func (h *trustMarkInstanceHandlers) counts(c fiber.Ctx) error {
	active, revoked, err := h.storages.TrustMarkInstances.CountByState(time.Now().Unix())
	if err != nil {
		return writeServerError(c, err)
	}
	return c.JSON(fiber.Map{"active": active, "revoked": revoked})
}

// revoke revokes every unrevoked instance of a trust mark type issued to an
// entity, whether or not the entity is a subject of a spec, and records a
// trust_mark_revoked event if any was revoked. It refuses an entity that is
// an active subject of the type: the trust mark endpoint would issue the mark
// again, so the subject's status is what to change.
func (h *trustMarkInstanceHandlers) revoke(c fiber.Ctx) error {
	var req struct {
		TrustMarkType string `json:"trust_mark_type"`
		Sub           string `json:"sub"`
		Message       string `json:"message"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return writeBadRequest(c, err.Error())
	}
	message, err := parseEventMessage(req.Message)
	if err != nil {
		return writeBadRequest(c, err.Error())
	}
	if req.TrustMarkType == "" || req.Sub == "" {
		return writeBadRequest(c, "trust_mark_type and sub are required")
	}
	defer h.locks.lock(req.TrustMarkType, req.Sub)()

	var revoked int64
	err = h.storages.InTransaction(
		func(tx *model.Backends) error {
			// The revocation is the first statement, so the active-subject
			// check below reads under the transaction's write lock; an
			// active subject rolls the revocation back.
			var err error
			if revoked, err = tx.TrustMarkInstances.RevokeByTypeAndSubject(req.TrustMarkType, req.Sub); err != nil {
				return err
			}
			active, err := hasActiveSubject(tx, req.TrustMarkType, req.Sub)
			if err != nil {
				return err
			}
			if active {
				return errSubjectActive
			}
			if revoked == 0 {
				return nil
			}
			return recordTrustMarkEvent(tx, c, model.EventTypeTrustMarkRevoked, req.TrustMarkType, req.Sub, message)
		},
	)
	if err != nil {
		if ok, werr := writeIssuanceError(c, err); ok {
			return werr
		}
		return writeServerError(c, err)
	}
	if revoked > 0 && h.cacheInvalidator != nil {
		h.cacheInvalidator.InvalidateTrustMark(req.TrustMarkType, req.Sub)
	}
	return c.JSON(fiber.Map{"revoked": revoked})
}

// hasActiveSubject reports whether entityID is an active subject of the spec
// of trustMarkType.
func hasActiveSubject(tx *model.Backends, trustMarkType, entityID string) (bool, error) {
	if tx.TrustMarkSpecs == nil {
		return false, nil
	}
	spec, err := tx.TrustMarkSpecs.GetByType(trustMarkType)
	if err != nil {
		if _, ok := errors.AsType[model.NotFoundError](err); ok {
			return false, nil
		}
		return false, err
	}
	subject, err := tx.TrustMarkSpecs.GetSubject(strconv.FormatUint(uint64(spec.ID), 10), entityID)
	if err != nil {
		if _, ok := errors.AsType[model.NotFoundError](err); ok {
			return false, nil
		}
		return false, err
	}
	return subject.EntityID == entityID && subject.Status == model.StatusActive, nil
}

// registerTrustMarkInstances registers the issued trust mark instance endpoints.
func registerTrustMarkInstances(r fiber.Router, storages model.Backends, cacheInvalidator TrustMarkCacheInvalidator) {
	h := &trustMarkInstanceHandlers{storages: storages, cacheInvalidator: cacheInvalidator, locks: &trustMarkLocks}
	r.Get("/trust-marks/instances", h.list)
	r.Get("/trust-marks/instances/counts", h.counts)
	r.Post("/trust-marks/instances/revoke", h.revoke)
}
