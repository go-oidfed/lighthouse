package adminapi

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// doRequest executes an HTTP request against a Fiber app and returns the response and body.
func doRequest(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Request %s %s failed: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	return resp, body
}
