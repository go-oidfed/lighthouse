package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJWKSSyncConfValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode JWKSSyncMode
		ok   bool
	}{
		{"empty defaults to none", "", true},
		{"none", JWKSSyncNone, true},
		{"push", JWKSSyncPush, true},
		{"trigger", JWKSSyncTrigger, true},
		{"unknown", "bogus", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (&JWKSSyncConf{Mode: tc.mode}).Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() err=%v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestAuthorityHintJWKSSyncJSONRoundTrip(t *testing.T) {
	in := AuthorityHint{
		ID:          1,
		EntityID:    "https://superior.example",
		Description: "a superior",
		JWKSSync: &JWKSSyncConf{
			Mode:        JWKSSyncTrigger,
			Timeout:     20,
			JWTLifetime: 600,
			Headers:     map[string]string{"X-Custom": "yes"},
		},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out AuthorityHint
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.JWKSSync == nil {
		t.Fatal("expected jwks_sync to be present")
	}
	if out.JWKSSync.Mode != JWKSSyncTrigger {
		t.Fatalf("mode = %q, want trigger", out.JWKSSync.Mode)
	}
	if out.JWKSSync.Timeout != 20 {
		t.Fatalf("timeout = %d, want 20", out.JWKSSync.Timeout)
	}
	if out.JWKSSync.JWTLifetime != 600 {
		t.Fatalf("jwt_lifetime = %d, want 600", out.JWKSSync.JWTLifetime)
	}
	if out.JWKSSync.Headers["X-Custom"] != "yes" {
		t.Fatalf("headers = %v", out.JWKSSync.Headers)
	}
}

func TestAuthorityHintJWKSSyncOmitted(t *testing.T) {
	in := AuthorityHint{ID: 2, EntityID: "https://superior.example"}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty JSON")
	}
	// jwks_sync must be omitted (nil pointer + omitempty), never serialized as
	// an empty object.
	if got := string(raw); strings.Contains(got, `"jwks_sync"`) {
		t.Fatalf("expected jwks_sync omitted, got %s", got)
	}

	var out AuthorityHint
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.JWKSSync != nil {
		t.Fatalf("expected nil jwks_sync, got %+v", out.JWKSSync)
	}
}
