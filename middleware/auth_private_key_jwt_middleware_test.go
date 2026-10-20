package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage"
)

// stubFedEntityForMiddlewareTest implements oidfed.FederationEntity as a bare
// stub for middleware tests that never reach entity configuration signing.
type stubFedEntityForMiddlewareTest struct{}

func (stubFedEntityForMiddlewareTest) EntityID() string { return "https://op.example.org" }
func (stubFedEntityForMiddlewareTest) EntityConfigurationPayload() (*oidfed.EntityStatementPayload, error) {
	return nil, nil
}
func (stubFedEntityForMiddlewareTest) EntityConfigurationJWT() ([]byte, error) { return nil, nil }
func (stubFedEntityForMiddlewareTest) SignEntityStatement(oidfed.EntityStatementPayload) ([]byte, error) {
	return nil, nil
}
func (stubFedEntityForMiddlewareTest) SignEntityStatementWithHeaders(
	oidfed.EntityStatementPayload, jws.Headers,
) ([]byte, error) {
	return nil, nil
}

// TestPrivateKeyJWTAuth_Middleware_ConventionalStack verifies that Middleware()
// works in a conventional fiber route stack: on a failed authentication it
// writes the OAuth error response and stops the chain (the protected handler
// must not run).
func TestPrivateKeyJWTAuth_Middleware_ConventionalStack(t *testing.T) {
	// The auth middleware rejects every request in this test: the assertion is
	// malformed, so the failure path is exercised deterministically.
	auth, err := NewPrivateKeyJWTAuth(
		"https://op.example.org",
		stubFedEntityForMiddlewareTest{},
		[]string{"https://ta.example.org"},
		func([]string) (anchors oidfed.TrustAnchors) { return nil },
		storage.NewJTICacheStorage(),
	)
	require.NoError(t, err)

	handlerRan := false
	app := fiber.New()
	app.Post("/protected", auth.Middleware(), func(c *fiber.Ctx) error {
		handlerRan = true
		return c.SendString("secret")
	})

	req := httptest.NewRequest(
		"POST", "/protected",
		strings.NewReader("client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer&client_assertion=not-a-jwt"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.False(t, handlerRan, "handler must not run when authentication fails")
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
