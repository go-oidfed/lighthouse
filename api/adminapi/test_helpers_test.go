package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
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

// requireStatus checks the response status code and calls t.Fatalf if it doesn't match.
// Use this when subsequent code depends on the correct status (e.g., body parsing follows).
func requireStatus(t *testing.T, resp *http.Response, body []byte, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, fmtBody(body))
	}
}

// assertStatus checks the response status code and calls t.Errorf if it doesn't match.
// Use this when the check is the final assertion or when you want to see all failures.
func assertStatus(t *testing.T, resp *http.Response, body []byte, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, fmtBody(body))
	}
}

// assertStatusOneOf checks that the response status code is one of the expected values.
func assertStatusOneOf(t *testing.T, resp *http.Response, expected ...int) {
	t.Helper()
	if slices.Contains(expected, resp.StatusCode) {
		return
	}
	t.Errorf("Expected status one of %v, got %d", expected, resp.StatusCode)
}

// assertErrorResponse checks the HTTP status code and verifies the JSON error body
// contains the expected "error" field (e.g., "invalid_request", "not_found", "server_error").
// Use this for error path tests instead of assertStatus when you want to verify the full API contract.
func assertErrorResponse(t *testing.T, resp *http.Response, body []byte, expectedStatus int, expectedError string) {
	t.Helper()
	assertStatus(t, resp, body, expectedStatus)

	var errBody struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &errBody); err != nil {
		t.Errorf("Failed to unmarshal error response: %v (body: %s)", err, fmtBody(body))
		return
	}
	if errBody.Error != expectedError {
		t.Errorf("Expected error type %q, got %q (description: %q)", expectedError, errBody.Error, errBody.ErrorDescription)
	}
}

// requireStatusMsg checks the response status code with a custom message prefix and calls t.Fatalf.
func requireStatusMsg(t *testing.T, resp *http.Response, body []byte, expected int, msg string) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("%s: expected status %d, got %d. Body: %s", msg, expected, resp.StatusCode, fmtBody(body))
	}
}

// fmtBody is a helper to format body bytes for error messages.
func fmtBody(body []byte) string {
	if len(body) > 500 {
		return string(body[:500]) + fmt.Sprintf("... (%d bytes total)", len(body))
	}
	return string(body)
}
