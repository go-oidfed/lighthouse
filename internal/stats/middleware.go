package stats

import (
	"encoding/json"
	"hash/fnv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// MiddlewareConfig contains configuration for the stats middleware.
type MiddlewareConfig struct {
	// CaptureClientIP enables capturing the client IP address.
	CaptureClientIP bool

	// CaptureUserAgent enables capturing the User-Agent header.
	CaptureUserAgent bool

	// CaptureQueryParams enables capturing URL query parameters.
	CaptureQueryParams bool

	// GeoIP provides country lookup from IP addresses. Can be nil.
	GeoIP GeoIPProvider

	// TrackedEndpoints is a map of endpoints to track. If empty, all requests are tracked.
	// Keys should be normalized endpoint names (e.g., "well-known", "fetch", "resolve").
	TrackedEndpoints map[string]bool

	// Buffer is the ring buffer to write entries to.
	Buffer *RingBuffer
}

// Middleware creates a Fiber middleware that captures request statistics.
// The middleware runs after the handler to capture response status and timing.
func Middleware(cfg MiddlewareConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Check if we should track this endpoint
		endpointName := normalizeEndpoint(path)
		if !shouldTrack(endpointName, cfg.TrackedEndpoints) {
			return c.Next()
		}

		// Capture start time
		start := time.Now()

		// Execute the handler
		err := c.Next()

		// Capture stats after response
		duration := time.Since(start)
		entry := &RequestLog{
			Timestamp:    start,
			Endpoint:     endpointName,
			Method:       c.Method(),
			StatusCode:   c.Response().StatusCode(),
			DurationMs:   int(duration.Milliseconds()),
			ResponseSize: len(c.Response().Body()),
			RequestSize:  len(c.Request().Body()),
		}

		// Capture client IP
		if cfg.CaptureClientIP {
			entry.ClientIP = c.IP()

			// GeoIP lookup if available
			if cfg.GeoIP != nil && entry.ClientIP != "" {
				entry.CountryCode = cfg.GeoIP.LookupCountry(entry.ClientIP)
			}
		}

		// Capture User-Agent
		if cfg.CaptureUserAgent {
			entry.UserAgent = string(c.Request().Header.UserAgent())
			if entry.UserAgent != "" {
				entry.UserAgentHash = hashString(entry.UserAgent)
			}
		}

		// Capture query parameters
		if cfg.CaptureQueryParams {
			params := captureQueryParams(c)
			if len(params) > 0 {
				if jsonBytes, err := json.Marshal(params); err == nil {
					entry.QueryParams = jsonBytes
				}
			}
		}

		// Capture error type if present
		if err != nil {
			entry.ErrorType = categorizeError(err, entry.StatusCode)
		} else if entry.StatusCode >= 400 {
			entry.ErrorType = categorizeStatusCode(entry.StatusCode)
		}

		// Write to buffer (non-blocking)
		cfg.Buffer.Write(entry)

		return err
	}
}

// normalizeEndpoint converts a path to a normalized endpoint name.
func normalizeEndpoint(path string) string {
	// Remove leading slash and lowercase
	path = strings.TrimPrefix(path, "/")
	path = strings.ToLower(path)
	return path
}

// shouldTrack determines if an endpoint should be tracked.
func shouldTrack(endpoint string, tracked map[string]bool) bool {
	// If no filter, track all non-API endpoints
	if len(tracked) == 0 {
		// Exclude API endpoints by default
		return !strings.HasPrefix(endpoint, "api")
	}

	return tracked[endpoint]
}

// captureQueryParams extracts query parameters from the request.
func captureQueryParams(c *fiber.Ctx) map[string]string {
	params := make(map[string]string)

	queryArgs := c.Request().URI().QueryArgs()
	for key, value := range queryArgs.All() {
		params[string(key)] = string(value)
	}

	return params
}

// hashString computes a FNV-1a hash of a string for efficient grouping.
func hashString(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// categorizeError converts an error to an error type string.
func categorizeError(err error, statusCode int) string {
	if err == nil {
		return ""
	}

	// Check for common fiber errors
	if fe, ok := err.(*fiber.Error); ok {
		switch fe.Code {
		case fiber.StatusNotFound:
			return "not_found"
		case fiber.StatusBadRequest:
			return "bad_request"
		case fiber.StatusUnauthorized:
			return "unauthorized"
		case fiber.StatusForbidden:
			return "forbidden"
		case fiber.StatusInternalServerError:
			return "internal_error"
		default:
			return categorizeStatusCode(fe.Code)
		}
	}

	return categorizeStatusCode(statusCode)
}

// categorizeStatusCode returns an error type based on HTTP status code.
func categorizeStatusCode(code int) string {
	switch {
	case code >= 500:
		return "server_error"
	case code == 404:
		return "not_found"
	case code == 400:
		return "bad_request"
	case code == 401:
		return "unauthorized"
	case code == 403:
		return "forbidden"
	case code >= 400:
		return "client_error"
	default:
		return ""
	}
}

// BuildTrackedEndpoints converts a list of endpoint path patterns to a map of endpoint names.
func BuildTrackedEndpoints(paths []string) map[string]bool {
	if len(paths) == 0 {
		return nil // Track all
	}

	tracked := make(map[string]bool)
	for _, path := range paths {
		name := normalizeEndpoint(path)
		tracked[name] = true
	}
	return tracked
}
