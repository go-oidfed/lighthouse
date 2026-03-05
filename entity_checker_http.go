package lighthouse

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/jwx"
)

// HTTPListEntityChecker fetches a list of entity IDs from an HTTP endpoint
// and checks if the requesting entity is in the list.
// The endpoint should return a JSON array of entity ID strings.
type HTTPListEntityChecker struct {
	URL      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`       // GET or POST, default GET
	Headers  map[string]string `yaml:"headers" json:"headers"`     // Additional headers
	Timeout  int               `yaml:"timeout" json:"timeout"`     // seconds, default 30
	CacheTTL int               `yaml:"cache_ttl" json:"cache_ttl"` // seconds, default 60

	mu        sync.RWMutex
	cache     []string
	cacheTime time.Time
}

// Check implements the EntityChecker interface
func (c *HTTPListEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	_ []string,
) (bool, int, *oidfed.Error) {
	list, err := c.getList()
	if err != nil {
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("failed to fetch entity list: " + err.Error())
	}

	if slices.Contains(list, entityConfiguration.Subject) {
		return true, 0, nil
	}
	return false, fiber.StatusForbidden,
		&oidfed.Error{
			Error:            "forbidden",
			ErrorDescription: "entity not in allowed list",
		}
}

func (c *HTTPListEntityChecker) getList() ([]string, error) {
	ttl := time.Duration(c.CacheTTL) * time.Second
	if ttl == 0 {
		ttl = 60 * time.Second
	}

	c.mu.RLock()
	if c.cache != nil && time.Since(c.cacheTime) < ttl {
		defer c.mu.RUnlock()
		return c.cache, nil
	}
	c.mu.RUnlock()

	// Fetch fresh list
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cache != nil && time.Since(c.cacheTime) < ttl {
		return c.cache, nil
	}

	list, err := c.fetchList()
	if err != nil {
		return nil, err
	}
	c.cache = list
	c.cacheTime = time.Now()
	return list, nil
}

func (c *HTTPListEntityChecker) fetchList() ([]string, error) {
	method := c.Method
	if method == "" {
		method = http.MethodGet
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequest(method, c.URL, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP %d from entity list endpoint", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	var list []string
	if err = json.Unmarshal(body, &list); err != nil {
		return nil, errors.Wrap(err, "invalid JSON response from entity list endpoint")
	}
	return list, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *HTTPListEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	// Use a struct without the mutex to avoid copying lock
	type AliasFields struct {
		URL      string            `yaml:"url" json:"url"`
		Method   string            `yaml:"method" json:"method"`
		Headers  map[string]string `yaml:"headers" json:"headers"`
		Timeout  int               `yaml:"timeout" json:"timeout"`
		CacheTTL int               `yaml:"cache_ttl" json:"cache_ttl"`
	}
	var alias AliasFields
	if err := node.Decode(&alias); err != nil {
		return errors.WithStack(err)
	}
	c.URL = alias.URL
	c.Method = alias.Method
	c.Headers = alias.Headers
	c.Timeout = alias.Timeout
	c.CacheTTL = alias.CacheTTL
	return nil
}

// JWTVerificationMode defines how JWT signatures are verified
type JWTVerificationMode string

const (
	// JWTVerificationModeJWKS verifies using a configured JWKS
	JWTVerificationModeJWKS JWTVerificationMode = "jwks"
	// JWTVerificationModeTrustAnchor verifies by building a trust chain to trust anchors
	JWTVerificationModeTrustAnchor JWTVerificationMode = "trust_anchor"
)

// JWTVerification configures how JWT signatures are verified
type JWTVerification struct {
	Mode         JWTVerificationMode `yaml:"mode" json:"mode"`
	JWKS         *jwx.JWKS           `yaml:"jwks" json:"jwks,omitempty"`
	TrustAnchors oidfed.TrustAnchors `yaml:"trust_anchors" json:"trust_anchors,omitempty"`
}

// HTTPListJWTEntityChecker fetches a signed JWT containing entity IDs from an HTTP endpoint.
// The JWT signature is verified either using a configured JWKS or by building a trust chain
// to federation trust anchors.
type HTTPListJWTEntityChecker struct {
	URL          string            `yaml:"url" json:"url"`
	Method       string            `yaml:"method" json:"method"`
	Headers      map[string]string `yaml:"headers" json:"headers"`
	Timeout      int               `yaml:"timeout" json:"timeout"`
	CacheTTL     int               `yaml:"cache_ttl" json:"cache_ttl"`
	ListClaim    string            `yaml:"list_claim" json:"list_claim"` // default "entities"
	Verification JWTVerification   `yaml:"verification" json:"verification"`

	mu        sync.RWMutex
	cache     []string
	cacheTime time.Time
}

// Check implements the EntityChecker interface
func (c *HTTPListJWTEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	_ []string,
) (bool, int, *oidfed.Error) {
	list, err := c.getList()
	if err != nil {
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("failed to fetch/verify entity list: " + err.Error())
	}

	if slices.Contains(list, entityConfiguration.Subject) {
		return true, 0, nil
	}
	return false, fiber.StatusForbidden,
		&oidfed.Error{
			Error:            "forbidden",
			ErrorDescription: "entity not in allowed list",
		}
}

func (c *HTTPListJWTEntityChecker) getList() ([]string, error) {
	ttl := time.Duration(c.CacheTTL) * time.Second
	if ttl == 0 {
		ttl = 60 * time.Second
	}

	c.mu.RLock()
	if c.cache != nil && time.Since(c.cacheTime) < ttl {
		defer c.mu.RUnlock()
		return c.cache, nil
	}
	c.mu.RUnlock()

	// Fetch fresh list
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cache != nil && time.Since(c.cacheTime) < ttl {
		return c.cache, nil
	}

	list, err := c.fetchAndVerifyList()
	if err != nil {
		return nil, err
	}
	c.cache = list
	c.cacheTime = time.Now()
	return list, nil
}

func (c *HTTPListJWTEntityChecker) fetchAndVerifyList() ([]string, error) {
	// Fetch JWT from URL
	jwtString, err := c.fetchJWT()
	if err != nil {
		return nil, err
	}

	// Verify signature based on mode
	var token jwt.Token
	switch c.Verification.Mode {
	case JWTVerificationModeJWKS:
		token, err = c.verifyWithJWKS(jwtString)
	case JWTVerificationModeTrustAnchor:
		token, err = c.verifyWithTrustAnchor(jwtString)
	default:
		return nil, errors.New("invalid verification mode: must be 'jwks' or 'trust_anchor'")
	}
	if err != nil {
		return nil, err
	}

	// Extract list from claims
	listClaim := c.ListClaim
	if listClaim == "" {
		listClaim = "entities"
	}

	var rawList any
	if err := token.Get(listClaim, &rawList); err != nil {
		return nil, errors.Errorf("claim '%s' not found in JWT", listClaim)
	}

	// Convert to string slice
	return toStringSlice(rawList)
}

func (c *HTTPListJWTEntityChecker) fetchJWT() (string, error) {
	method := c.Method
	if method == "" {
		method = http.MethodGet
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequest(method, c.URL, http.NoBody)
	if err != nil {
		return "", errors.Wrap(err, "failed to create HTTP request")
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("HTTP %d from entity list endpoint", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	return string(body), nil
}

func (c *HTTPListJWTEntityChecker) verifyWithJWKS(jwtString string) (jwt.Token, error) {
	if c.Verification.JWKS == nil || c.Verification.JWKS.Set == nil {
		return nil, errors.New("JWKS not configured for JWT verification")
	}

	token, err := jwt.Parse([]byte(jwtString), jwt.WithKeySet(c.Verification.JWKS.Set))
	if err != nil {
		return nil, errors.Wrap(err, "JWT verification failed")
	}

	return token, nil
}

func (c *HTTPListJWTEntityChecker) verifyWithTrustAnchor(jwtString string) (jwt.Token, error) {
	if len(c.Verification.TrustAnchors) == 0 {
		return nil, errors.New("no trust anchors configured for JWT verification")
	}

	// Parse JWT without verification to get issuer
	token, err := jwt.Parse([]byte(jwtString), jwt.WithVerify(false))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse JWT")
	}
	issuer, ok := token.Issuer()
	if !ok || issuer == "" {
		return nil, errors.New("JWT has no issuer claim")
	}

	// Try each trust anchor
	var lastErr error
	for _, ta := range c.Verification.TrustAnchors {
		// Try to resolve trust path from issuer to trust anchor
		valid, _ := oidfed.DefaultMetadataResolver.ResolvePossible(
			apimodel.ResolveRequest{
				Subject:     issuer,
				TrustAnchor: []string{ta.EntityID},
			},
		)
		if !valid {
			lastErr = errors.Errorf("no valid trust path from %s to %s", issuer, ta.EntityID)
			continue
		}

		// Get the issuer's entity configuration to get their signing keys
		issuerConfig, err := oidfed.GetEntityConfiguration(issuer)
		if err != nil {
			lastErr = err
			continue
		}

		// Verify JWT with issuer's federation keys
		verifiedToken, err := jwt.Parse([]byte(jwtString), jwt.WithKeySet(issuerConfig.JWKS.Set))
		if err != nil {
			lastErr = err
			continue
		}

		return verifiedToken, nil
	}

	if lastErr != nil {
		return nil, errors.Wrap(lastErr, "could not verify JWT with any trust anchor")
	}
	return nil, errors.New("could not verify JWT with any trust anchor")
}

// toStringSlice converts an interface{} to []string
func toStringSlice(v any) ([]string, error) {
	switch val := v.(type) {
	case []string:
		return val, nil
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New("list contains non-string elements")
			}
			result = append(result, str)
		}
		return result, nil
	default:
		return nil, errors.Errorf("expected array, got %T", v)
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *HTTPListJWTEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	// Use a struct without the mutex to avoid copying lock
	type AliasFields struct {
		URL          string            `yaml:"url" json:"url"`
		Method       string            `yaml:"method" json:"method"`
		Headers      map[string]string `yaml:"headers" json:"headers"`
		Timeout      int               `yaml:"timeout" json:"timeout"`
		CacheTTL     int               `yaml:"cache_ttl" json:"cache_ttl"`
		ListClaim    string            `yaml:"list_claim" json:"list_claim"`
		Verification JWTVerification   `yaml:"verification" json:"verification"`
	}
	var alias AliasFields
	if err := node.Decode(&alias); err != nil {
		return errors.WithStack(err)
	}
	c.URL = alias.URL
	c.Method = alias.Method
	c.Headers = alias.Headers
	c.Timeout = alias.Timeout
	c.CacheTTL = alias.CacheTTL
	c.ListClaim = alias.ListClaim
	c.Verification = alias.Verification
	return nil
}

func init() {
	RegisterEntityChecker("http_list", func() EntityChecker { return &HTTPListEntityChecker{} })
	RegisterEntityChecker("http_list_jwt", func() EntityChecker { return &HTTPListJWTEntityChecker{} })
}
