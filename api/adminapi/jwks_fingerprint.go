package adminapi

import (
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/go-oidfed/lib/jwx"
)

// fingerprintPrefixLen is how many hex digits of a key set's fingerprint a
// history message shows.
const fingerprintPrefixLen = 12

// jwksFingerprint identifies a key set by its public key material, whatever
// the order of its keys: the hex SHA-256 of the keys' RFC 7638 SHA-256
// thumbprints, each hex encoded, sorted and joined with "\n". It is "" for a
// nil or empty set, and also when a key's thumbprint cannot be computed. Only
// the public members that RFC 7638 uses count: private members, the key ID,
// alg and use do not change it.
func jwksFingerprint(keys jwx.JWKS) string {
	if keys.Set == nil || keys.Len() == 0 {
		return ""
	}
	thumbprints := make([]string, 0, keys.Len())
	for i := 0; i < keys.Len(); i++ {
		k, ok := keys.Key(i)
		if !ok {
			return ""
		}
		tp, err := k.Thumbprint(crypto.SHA256)
		if err != nil {
			return ""
		}
		thumbprints = append(thumbprints, hex.EncodeToString(tp))
	}
	slices.Sort(thumbprints)
	sum := sha256.Sum256([]byte(strings.Join(thumbprints, "\n")))
	return hex.EncodeToString(sum[:])
}

// keysStoredMessage is the history message for storing the key set stored in
// place of old: how many keys, the start of the stored set's fingerprint and
// whether it differs from old. An empty set has no fingerprint and is
// unchanged only from an empty set; a non-empty set whose fingerprint cannot
// be computed is never unchanged.
func keysStoredMessage(old, stored jwx.JWKS) string {
	n := 0
	if stored.Set != nil {
		n = stored.Len()
	}
	oldEmpty := old.Set == nil || old.Len() == 0
	fp := jwksFingerprint(stored)
	unchanged := (n == 0 && oldEmpty) || (fp != "" && fp == jwksFingerprint(old))
	change := "changed"
	if unchanged {
		change = "unchanged"
	}
	shown := "no fingerprint"
	if fp != "" {
		shown = "fingerprint " + fp[:fingerprintPrefixLen]
	}
	noun := "keys"
	if n == 1 {
		noun = "key"
	}
	return fmt.Sprintf("stored %d %s (%s, %s)", n, noun, shown, change)
}
