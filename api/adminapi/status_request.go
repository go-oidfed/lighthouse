package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/lestrrat-go/jwx/v4/jwk"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// maxStatusMessageLen is the longest custom event message, in characters.
const maxStatusMessageLen = 1000

// errStatusConflict reports that a status change found the status other than
// the one it expected.
var errStatusConflict = errors.New("the status is not the expected one")

// statusRequest is a status change. A request with the JSON content type has
// an object body; any other request has a plain text body holding the status.
type statusRequest struct {
	Status model.Status
	// Object is true for an object body. The other fields are set only then.
	Object bool
	// ExpectedStatus, when not nil, is the status the target must have.
	ExpectedStatus *model.Status
	// Message replaces the default message of the status event.
	Message string
	// JWKS replaces the keys of a subordinate.
	JWKS *model.JWKS
}

// statusTarget is the kind of entity a status change applies to. It decides
// which fields an object body may carry.
type statusTarget int

const (
	// subordinateStatus is the status of a subordinate.
	subordinateStatus statusTarget = iota
	// trustMarkSubjectStatus is the status of a trust mark subject.
	trustMarkSubjectStatus
)

// parseStatusRequest reads the status change of a target in c. A request that
// is invalid, also one with a field that does not apply to the target, is
// answered with the returned error as a 400.
func parseStatusRequest(c fiber.Ctx, target statusTarget) (*statusRequest, error) {
	req := &statusRequest{}
	if !isObjectBody(c) {
		var err error
		req.Status, err = parseStatusValue(string(c.Body()))
		return req, err
	}
	req.Object = true
	var body struct {
		Status         string          `json:"status"`
		ExpectedStatus string          `json:"expected_status"`
		Message        string          `json:"message"`
		JWKS           json.RawMessage `json:"jwks"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return nil, errors.New("invalid body")
	}
	if body.JWKS != nil && target != subordinateStatus {
		return nil, errors.New("jwks is not supported on a trust mark subject status change")
	}
	var err error
	if req.Status, err = parseStatusValue(body.Status); err != nil {
		return nil, err
	}
	if body.ExpectedStatus != "" {
		expected, err := model.ParseStatus(body.ExpectedStatus)
		if err != nil {
			return nil, fmt.Errorf("expected_status: %w", err)
		}
		req.ExpectedStatus = &expected
	}
	req.Message = strings.TrimSpace(body.Message)
	if utf8.RuneCountInString(req.Message) > maxStatusMessageLen {
		return nil, fmt.Errorf("message is longer than %d characters", maxStatusMessageLen)
	}
	if len(body.JWKS) > 0 && string(body.JWKS) != "null" {
		var jwks model.JWKS
		if err = json.Unmarshal(body.JWKS, &jwks); err != nil {
			return nil, fmt.Errorf("invalid jwks: %w", err)
		}
		if msg := unsupportedKeyMessage(&jwks); msg != "" {
			return nil, errors.New(msg)
		}
		req.JWKS = &jwks
	}
	return req, nil
}

// isObjectBody reports whether c carries an object body: a JSON object with
// the JSON content type. A plain status value is never an object, so one sent
// under the JSON content type (as older clients do) stays a plain text body.
func isObjectBody(c fiber.Ctx) bool {
	mediaType, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	return err == nil && mediaType == fiber.MIMEApplicationJSON &&
		strings.HasPrefix(strings.TrimSpace(string(c.Body())), "{")
}

func parseStatusValue(s string) (model.Status, error) {
	if s = strings.TrimSpace(s); s == "" {
		return model.StatusPending, errors.New("status is required")
	}
	return model.ParseStatus(s)
}

// expects reports whether a target in status current satisfies the expected
// status of req, if it has one.
func (r *statusRequest) expects(current model.Status) bool {
	return r.ExpectedStatus == nil || *r.ExpectedStatus == current
}

// unsupportedKeyMessage describes the first key of jwks that is not a
// supported JWK, or is empty if there is none.
func unsupportedKeyMessage(jwks *model.JWKS) string {
	if jwks.Keys.Set == nil {
		return ""
	}
	for _, k := range jwks.Keys.All() {
		if jwk.IsUnsupportedKey(k) {
			if uk, ok := k.(jwk.UnsupportedKey); ok {
				return "invalid JWK: " + uk.Reason().Error()
			}
			return "invalid JWK in set"
		}
	}
	return ""
}
