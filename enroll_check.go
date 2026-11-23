package lighthouse

import (
	"context"
	"sync"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/rs/zerolog/log"

	"github.com/go-oidfed/lighthouse/api/adminapi"
)

// enrollCheckTimeout (a variable for tests) bounds how long an enroll check
// waits for the entity configuration of the checked entity.
var enrollCheckTimeout = 20 * time.Second

// maxEnrollResolutions is how many entity configuration resolutions of
// enroll checks may run at once.
const maxEnrollResolutions = 8

// setEnrollChecker records the entity checker of the enroll endpoint that is
// now registered, or nil when there is none. While endpoints are being loaded
// from the database (see beginEnrollCheckerLoad) it is staged and takes
// effect when the load is done.
func (fed *LightHouse) setEnrollChecker(checker EntityChecker) {
	fed.enrollCheckerMu.Lock()
	defer fed.enrollCheckerMu.Unlock()
	if fed.enrollCheckerLoading {
		fed.stagedEnrollChecker = checker
		return
	}
	fed.enrollChecker = checker
}

// beginEnrollCheckerLoad starts collecting the checker of the endpoints that
// are loaded next, leaving the current checker in place.
func (fed *LightHouse) beginEnrollCheckerLoad() {
	fed.enrollCheckerMu.Lock()
	defer fed.enrollCheckerMu.Unlock()
	fed.enrollCheckerLoading = true
	fed.stagedEnrollChecker = nil
}

// endEnrollCheckerLoad makes the checker collected since beginEnrollCheckerLoad
// the current one, nil if no enroll endpoint with a checker was loaded.
func (fed *LightHouse) endEnrollCheckerLoad() {
	fed.enrollCheckerMu.Lock()
	defer fed.enrollCheckerMu.Unlock()
	fed.enrollChecker, fed.stagedEnrollChecker = fed.stagedEnrollChecker, nil
	fed.enrollCheckerLoading = false
}

// currentEnrollChecker returns the entity checker of the registered enroll
// endpoint, or nil if there is no such endpoint or it has no checker.
func (fed *LightHouse) currentEnrollChecker() EntityChecker {
	fed.enrollCheckerMu.RLock()
	defer fed.enrollCheckerMu.RUnlock()
	return fed.enrollChecker
}

// enrollResolutions runs entity configuration resolutions for enroll checks.
// The resolver takes no context, so a resolution cannot be cancelled: it
// runs to the end even if everyone stopped waiting. To keep those from piling
// up, concurrent checks of the same entity share one resolution, and at most
// max resolutions run at once.
type enrollResolutions struct {
	resolve  func(entityID string, types []string) *oidfed.EntityStatement
	max      int
	mu       sync.Mutex
	inflight map[string]*enrollResolution
}

type enrollResolution struct {
	done chan struct{}
	ec   *oidfed.EntityStatement
}

func newEnrollResolutions(
	max int, resolve func(string, []string) *oidfed.EntityStatement,
) *enrollResolutions {
	return &enrollResolutions{resolve: resolve, max: max, inflight: map[string]*enrollResolution{}}
}

// defaultEnrollResolutions is shared by all admin enroll checks.
var defaultEnrollResolutions = newEnrollResolutions(maxEnrollResolutions, resolveEnrollEntityConfiguration)

// start returns the running resolution of entityID with types, or starts one.
// It returns adminapi.ErrEnrollCheckBusy if starting one would exceed the cap.
func (r *enrollResolutions) start(entityID string, types []string) (*enrollResolution, error) {
	key := entityID
	for _, t := range types {
		key += "\x00" + t
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if res, ok := r.inflight[key]; ok {
		return res, nil
	}
	if len(r.inflight) >= r.max {
		return nil, adminapi.ErrEnrollCheckBusy
	}
	res := &enrollResolution{done: make(chan struct{})}
	r.inflight[key] = res
	go func() {
		var ec *oidfed.EntityStatement
		// The library panics on some entity configurations (for example one
		// without metadata when types are requested). In the enroll endpoint
		// that is a recovered 500; here it would end the process, so the
		// entity configuration counts as not obtainable.
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().Interface("panic", rec).Str("entity_id", entityID).
					Msg("enroll check: resolving the entity configuration panicked")
				ec = nil
			}
			r.mu.Lock()
			res.ec = ec
			delete(r.inflight, key)
			r.mu.Unlock()
			close(res.done)
		}()
		ec = r.resolve(entityID, types)
	}()
	return res, nil
}

// adminEnrollChecker implements adminapi.EnrollChecker. It reads the current
// checker at call time, as the admin API is registered before the endpoints
// and endpoints can be reloaded.
type adminEnrollChecker struct {
	fed *LightHouse
	// resolutions defaults to defaultEnrollResolutions.
	resolutions *enrollResolutions
}

var _ adminapi.EnrollChecker = adminEnrollChecker{}

// Configured reports whether an enroll endpoint with an entity checker is
// currently registered.
func (a adminEnrollChecker) Configured() bool {
	return a.fed.currentEnrollChecker() != nil
}

// CheckEnroll obtains the entity configuration of entityID and runs the
// enroll endpoint's entity checker on it, as a request to the enroll endpoint
// would, but without enrolling anything. Without a checker nothing is fetched
// and the result has Configured false. A configuration that cannot be
// obtained within enrollCheckTimeout or verified, or that has no metadata to
// guess entity types from, is reported in the result, with OK false, and the
// checker does not run. It returns adminapi.ErrEnrollCheckBusy when too many
// resolutions are running.
func (a adminEnrollChecker) CheckEnroll(
	ctx context.Context, entityID string, entityTypes []string,
) (adminapi.EnrollCheckResult, error) {
	res := adminapi.EnrollCheckResult{EntityTypes: entityTypes}
	checker := a.fed.currentEnrollChecker()
	if checker == nil {
		return res, nil
	}
	res.Configured = true
	resolutions := a.resolutions
	if resolutions == nil {
		resolutions = defaultEnrollResolutions
	}
	resolution, err := resolutions.start(entityID, entityTypes)
	if err != nil {
		return res, err
	}
	ctx, cancel := context.WithTimeout(ctx, enrollCheckTimeout)
	defer cancel()
	select {
	case <-resolution.done:
	case <-ctx.Done():
		res.Reason = "the entity did not answer in time"
		return res, nil
	}
	entityConfig := resolution.ec
	if entityConfig == nil {
		res.Reason = "could not obtain or verify entity configuration"
		return res, nil
	}
	types, ok := enrollEntityTypes(entityTypes, entityConfig)
	if !ok {
		res.Reason = errMsgEnrollNoMetadata
		return res, nil
	}
	res.EntityTypes = types
	ok, _, errResponse := checker.Check(entityConfig, res.EntityTypes)
	if ok {
		res.OK = true
		return res, nil
	}
	res.Reason = "entity check failed"
	if errResponse != nil && errResponse.ErrorDescription != "" {
		res.Reason = errResponse.ErrorDescription
	}
	return res, nil
}
