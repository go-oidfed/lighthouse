package stats

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp builds a Fiber app wired with the stats middleware writing into
// the given buffer, mirroring how lighthouse.go attaches the collector.
func newTestApp(t *testing.T, buffer *RingBuffer) *fiber.App {
	t.Helper()
	app := fiber.New(
		fiber.Config{
			// Use the same error handler as the real server so the test reflects
			// production behaviour: the status is applied by the ErrorHandler
			// *after* the middleware chain returns.
			ErrorHandler: func(c *fiber.Ctx, err error) error {
				// Mirrors lighthouse.httpErrorHandler: the status is applied here,
				// *after* the middleware chain returns.
				code := fiber.StatusInternalServerError
				msg := err.Error()
				if fe, ok := errors.AsType[*fiber.Error](err); ok {
					code = fe.Code
					msg = fe.Error()
				}
				return c.Status(code).JSON(fiber.Map{"error": msg})
			},
		},
	)
	app.Use(
		Middleware(
			MiddlewareConfig{
				Buffer: buffer,
			},
		),
	)
	return app
}

func TestMiddleware_RecordsNotFoundStatusForUnmatchedRoute(t *testing.T) {
	buffer := NewRingBuffer(100, 0.8)
	app := newTestApp(t, buffer)
	// No routes are registered, so every request yields fiber.ErrNotFound.

	req := httptest.NewRequest(fiber.MethodGet, "/this-path-does-not-exist-at-all", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	// Sanity: the client really does receive a 404 (set by the ErrorHandler).
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()

	entries := buffer.Drain()
	require.Len(t, entries, 1, "expected exactly one stats entry")
	assert.Equal(
		t, fiber.StatusNotFound, entries[0].StatusCode,
		"middleware should record the effective 404, not fasthttp's default 200",
	)
	assert.Equal(t, "not_found", entries[0].ErrorType)
	assert.Equal(t, fiber.MethodGet, entries[0].Method)
	assert.Equal(t, "this-path-does-not-exist-at-all", entries[0].Endpoint)
}

func TestMiddleware_RecordsOKStatusForRegisteredRoute(t *testing.T) {
	buffer := NewRingBuffer(100, 0.8)
	app := newTestApp(t, buffer)
	app.Get(
		"/ping", func(c *fiber.Ctx) error {
			return c.SendString("pong")
		},
	)

	req := httptest.NewRequest(fiber.MethodGet, "/ping", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	entries := buffer.Drain()
	require.Len(t, entries, 1)
	assert.Equal(t, fiber.StatusOK, entries[0].StatusCode)
	assert.Empty(t, entries[0].ErrorType)
	assert.Equal(t, "ping", entries[0].Endpoint)
}

func TestMiddleware_RecordsExplicitFiberErrorStatus(t *testing.T) {
	buffer := NewRingBuffer(100, 0.8)
	app := newTestApp(t, buffer)
	app.Get(
		"/fail", func(*fiber.Ctx) error {
			return fiber.NewError(fiber.StatusServiceUnavailable, "downstream gone")
		},
	)

	req := httptest.NewRequest(fiber.MethodGet, "/fail", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()

	entries := buffer.Drain()
	require.Len(t, entries, 1)
	assert.Equal(t, fiber.StatusServiceUnavailable, entries[0].StatusCode)
	assert.Equal(t, "server_error", entries[0].ErrorType)
}

func TestMiddleware_RecordsGenericErrorAsInternalServerError(t *testing.T) {
	buffer := NewRingBuffer(100, 0.8)
	app := newTestApp(t, buffer)
	app.Get(
		"/boom", func(*fiber.Ctx) error {
			return assert.AnError // a non-fiber error
		},
	)

	req := httptest.NewRequest(fiber.MethodGet, "/boom", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	_ = resp.Body.Close()

	entries := buffer.Drain()
	require.Len(t, entries, 1)
	assert.Equal(t, fiber.StatusInternalServerError, entries[0].StatusCode)
	assert.Equal(t, "server_error", entries[0].ErrorType)
}
