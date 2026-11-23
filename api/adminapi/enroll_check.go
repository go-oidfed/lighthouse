package adminapi

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// ErrEnrollCheckBusy is returned by EnrollChecker.CheckEnroll when too many
// checks are already waiting for entity configurations. The route answers
// 503 enroll_check_busy.
var ErrEnrollCheckBusy = errors.New("too many enroll checks are running")

// EnrollChecker runs the running entity's enroll entity checker. LightHouse
// provides it in Options.EnrollChecker; the checker it runs is the one the
// enroll endpoint is configured with, on the entity configuration obtained
// the way the enroll endpoint obtains it, so the checker step decides as the
// enroll endpoint's does. (The enroll endpoint also answers for entities that
// are already subordinates before it checks anything; this route does not.)
type EnrollChecker interface {
	// Configured reports whether an enroll endpoint with an entity checker
	// is registered right now. An externally hosted enroll endpoint (empty
	// path) registers no checker. It is evaluated at call time, as endpoints
	// can be reloaded.
	Configured() bool
	// CheckEnroll obtains the entity configuration of entityID itself, within
	// a bounded time, and runs the checker on it. entityTypes are the entity
	// types to check; when empty, they are guessed from the entity's
	// metadata. An entity whose configuration cannot be obtained or verified
	// is a result with OK false and a Reason, not an error. When no checker
	// is configured, nothing is fetched and the result has Configured false.
	CheckEnroll(ctx context.Context, entityID string, entityTypes []string) (EnrollCheckResult, error)
}

// EnrollCheckResult is the outcome of EnrollChecker.CheckEnroll.
type EnrollCheckResult struct {
	// Configured is false when no enroll entity checker is configured.
	Configured bool
	// OK is whether the checker accepts the entity. It is false when
	// Configured is false.
	OK bool
	// Reason says why the check failed. It is empty when OK.
	Reason string
	// EntityTypes are the entity types that were checked.
	EntityTypes []string
}

type enrollCheckResponse struct {
	Configured  bool     `json:"configured"`
	OK          bool     `json:"ok"`
	Reason      string   `json:"reason,omitempty"`
	EntityTypes []string `json:"entity_types"`
}

// registerEnrollCheck mounts POST /subordinates/{subordinateID}/enroll-check.
// A nil checker behaves like a LightHouse without an enroll entity checker.
func registerEnrollCheck(r fiber.Router, subordinates model.SubordinateStorageBackend, checker EnrollChecker) {
	r.Post(
		"/subordinates/:subordinateID/enroll-check", func(c fiber.Ctx) error {
			sub, ok := handleSubordinateLookup(c, subordinates)
			if !ok {
				return nil
			}
			types := make([]string, 0, len(sub.SubordinateEntityTypes))
			for _, t := range sub.SubordinateEntityTypes {
				types = append(types, t.EntityType)
			}
			res := EnrollCheckResult{EntityTypes: types}
			if checker != nil {
				var err error
				res, err = checker.CheckEnroll(c.Context(), sub.EntityID, types)
				if errors.Is(err, ErrEnrollCheckBusy) {
					return writeCodedError(
						c, fiber.StatusServiceUnavailable, "enroll_check_busy", "too many enroll checks are running; retry later",
					)
				}
				if err != nil {
					return writeServerError(c, err)
				}
			}
			if res.EntityTypes == nil {
				res.EntityTypes = []string{}
			}
			return c.JSON(
				enrollCheckResponse{
					Configured: res.Configured, OK: res.OK, Reason: res.Reason, EntityTypes: res.EntityTypes,
				},
			)
		},
	)
}
