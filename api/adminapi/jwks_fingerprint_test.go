package adminapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/go-oidfed/lib/jwx"
	"github.com/lestrrat-go/jwx/v4/jwk"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// newECKey returns a fresh public EC JWK. Unlike createTestKey, every call
// yields different key material.
func newECKey(t *testing.T, kid string) jwk.Key {
	t.Helper()
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jwk.PublicKeyOf(sk)
	if err != nil {
		t.Fatal(err)
	}
	if err = pub.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatal(err)
	}
	return pub
}

func keySet(t *testing.T, keys ...jwk.Key) jwx.JWKS {
	t.Helper()
	set := jwk.NewSet()
	for _, k := range keys {
		if err := set.AddKey(k); err != nil {
			t.Fatal(err)
		}
	}
	return jwx.JWKS{Set: set}
}

var keysStoredRE = regexp.MustCompile(`^stored (\d+) keys? \(fingerprint ([0-9a-f]{12}), (changed|unchanged)\)$`)

// parseKeysStored splits a jwks_replaced message into its parts.
func parseKeysStored(t *testing.T, msg string) (n, fingerprint, change string) {
	t.Helper()
	m := keysStoredRE.FindStringSubmatch(msg)
	if m == nil {
		t.Fatalf("message %q does not match %s", msg, keysStoredRE)
	}
	return m[1], m[2], m[3]
}

// jwksReplacedMessages returns the messages of a subordinate's jwks_replaced
// events, oldest first.
func jwksReplacedMessages(t *testing.T, backends model.Backends, id uint) []string {
	t.Helper()
	events, _, err := backends.SubordinateEvents.GetBySubordinateID(id, model.EventQueryOpts{})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}
	var out []string
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != model.EventTypeJWKSReplaced {
			continue
		}
		if events[i].Message == nil {
			t.Fatal("jwks_replaced event has no message")
		}
		out = append(out, *events[i].Message)
	}
	return out
}

// checkReplacementSequence checks the messages recorded for the sequence
// of replacements a, a (other order), b: changed, unchanged, changed.
func checkReplacementSequence(t *testing.T, got []string, counts []string) {
	t.Helper()
	if len(got) != 3 {
		t.Fatalf("got %d jwks_replaced messages, want 3: %q", len(got), got)
	}
	var fps, changes []string
	for i, msg := range got {
		n, fp, change := parseKeysStored(t, msg)
		if n != counts[i] {
			t.Errorf("message %d counts %s keys, want %s", i, n, counts[i])
		}
		fps = append(fps, fp)
		changes = append(changes, change)
	}
	if want := []string{"changed", "unchanged", "changed"}; strings.Join(changes, ",") != strings.Join(want, ",") {
		t.Errorf("changes = %q, want %q", changes, want)
	}
	if fps[0] != fps[1] {
		t.Errorf("the same keys in another order give fingerprints %s and %s", fps[0], fps[1])
	}
	if fps[0] == fps[2] {
		t.Errorf("different keys give the same fingerprint %s", fps[0])
	}
}

func TestJWKSFingerprint(t *testing.T) {
	t.Parallel()
	a, b, c := newECKey(t, "a"), newECKey(t, "b"), newECKey(t, "c")

	ab, ba := jwksFingerprint(keySet(t, a, b)), jwksFingerprint(keySet(t, b, a))
	if len(ab) != 64 || ab != ba {
		t.Errorf("order must not matter: %q vs %q", ab, ba)
	}
	if ab == jwksFingerprint(keySet(t, a)) || ab == jwksFingerprint(keySet(t, a, c)) {
		t.Error("different key sets must differ")
	}
	// The key ID is not key material.
	renamed, err := a.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = renamed.Set(jwk.KeyIDKey, "other"); err != nil {
		t.Fatal(err)
	}
	if jwksFingerprint(keySet(t, a)) != jwksFingerprint(keySet(t, renamed)) {
		t.Error("the key ID must not change the fingerprint")
	}
	if jwksFingerprint(jwx.JWKS{}) != "" || jwksFingerprint(keySet(t)) != "" {
		t.Error("an empty set has no fingerprint")
	}
}

func TestJWKSFingerprintKeyMaterialOnly(t *testing.T) {
	t.Parallel()
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, err := jwk.Import[jwk.Key](sk)
	if err != nil {
		t.Fatal(err)
	}
	public, err := jwk.PublicKeyOf(sk)
	if err != nil {
		t.Fatal(err)
	}
	if !private.Has("d") {
		t.Fatal("test setup: the private key has no d")
	}
	if jwksFingerprint(keySet(t, private)) != jwksFingerprint(keySet(t, public)) ||
		jwksFingerprint(keySet(t, public)) == "" {
		t.Error("private members must not change the fingerprint")
	}

	tagged, err := public.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = tagged.Set(jwk.AlgorithmKey, "ES256"); err != nil {
		t.Fatal(err)
	}
	if err = tagged.Set(jwk.KeyUsageKey, "sig"); err != nil {
		t.Fatal(err)
	}
	if jwksFingerprint(keySet(t, tagged)) != jwksFingerprint(keySet(t, public)) {
		t.Error("alg and use must not change the fingerprint")
	}
	if got := keysStoredMessage(keySet(t, public), keySet(t, tagged)); !strings.HasSuffix(got, ", unchanged)") {
		t.Errorf("same material with another alg/use: %q", got)
	}
}

func TestKeysStoredMessage(t *testing.T) {
	t.Parallel()
	a, b := newECKey(t, "a"), newECKey(t, "b")
	ab := keySet(t, a, b)
	fp := func(set jwx.JWKS) string { return jwksFingerprint(set)[:12] }
	for _, tc := range []struct {
		name        string
		old, stored jwx.JWKS
		want        string
	}{
		{"first key", jwx.JWKS{}, keySet(t, a), fmt.Sprintf("stored 1 key (fingerprint %s, changed)", fp(keySet(t, a)))},
		{"same key", keySet(t, a), keySet(t, a), fmt.Sprintf("stored 1 key (fingerprint %s, unchanged)", fp(keySet(t, a)))},
		{"other key", keySet(t, a), keySet(t, b), fmt.Sprintf("stored 1 key (fingerprint %s, changed)", fp(keySet(t, b)))},
		{"two keys", keySet(t, a), ab, fmt.Sprintf("stored 2 keys (fingerprint %s, changed)", fp(ab))},
		{"empty over empty", jwx.JWKS{}, keySet(t), "stored 0 keys (no fingerprint, unchanged)"},
		{"nil over nil", jwx.JWKS{}, jwx.JWKS{}, "stored 0 keys (no fingerprint, unchanged)"},
		{"keys removed", keySet(t, a), keySet(t), "stored 0 keys (no fingerprint, changed)"},
	} {
		if got := keysStoredMessage(tc.old, tc.stored); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
