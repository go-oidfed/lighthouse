package adminapi

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"os"
	"runtime"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// minServiceTokenLen is the minimum length of a service token in bytes.
const minServiceTokenLen = 32

// ServiceToken is a bearer token accepted on the service listener. Only the
// SHA-256 hash of the token is kept.
type ServiceToken struct {
	// Name identifies the token. It is the actor recorded for requests made
	// with the token, unless the token acts for a user.
	Name string
	// ActForUsers lets requests made with the token name, in the X-Actor
	// header, the user they act for.
	ActForUsers bool
	hash        [sha256.Size]byte
}

// NewServiceToken returns the service token with the given name and value.
// The value must be at least 32 bytes long. Errors never contain the value.
func NewServiceToken(name string, secret []byte, actForUsers bool) (ServiceToken, error) {
	if name == "" {
		return ServiceToken{}, errors.New("service token: name must not be empty")
	}
	if len(secret) < minServiceTokenLen {
		return ServiceToken{}, errors.Errorf(
			"service token %q: the token must be at least %d bytes long", name, minServiceTokenLen,
		)
	}
	return ServiceToken{Name: name, ActForUsers: actForUsers, hash: sha256.Sum256(secret)}, nil
}

// LoadServiceToken reads a service token from file. Trailing newlines are
// removed; the rest must be at least 32 bytes long. Errors never contain the
// token.
//
// A warning is logged, naming the token and the file, when the file can be
// read by its group or by others.
func LoadServiceToken(name, file string, actForUsers bool) (ServiceToken, error) {
	return loadServiceToken(name, file, actForUsers, log.Logger)
}

// loadServiceToken is LoadServiceToken, logging to logger.
func loadServiceToken(name, file string, actForUsers bool, logger zerolog.Logger) (ServiceToken, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return ServiceToken{}, errors.Wrapf(err, "service token %q: cannot read token file", name)
	}
	if info, err := os.Stat(file); err == nil && runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		logger.Warn().
			Str("service_token", name).
			Str("token_file", file).
			Str("mode", info.Mode().Perm().String()).
			Msg("service token file is readable by its group or others; restrict it to its owner (chmod 600)")
	}
	return NewServiceToken(name, bytes.TrimRight(data, "\r\n"), actForUsers)
}

// matches reports, in constant time, whether presented is the token.
func (t ServiceToken) matches(presented []byte) bool {
	h := sha256.Sum256(presented)
	return subtle.ConstantTimeCompare(h[:], t.hash[:]) == 1
}

// ServiceTokens are the tokens accepted on the service listener.
type ServiceTokens []ServiceToken

// ActForUsers reports whether at least one of the tokens may act for users.
func (ts ServiceTokens) ActForUsers() bool {
	for _, t := range ts {
		if t.ActForUsers {
			return true
		}
	}
	return false
}

// find returns the token that presented matches. Every token is compared, so
// the time taken does not depend on which one matches.
func (ts ServiceTokens) find(presented []byte) (ServiceToken, bool) {
	var found ServiceToken
	ok := false
	for _, t := range ts {
		if t.matches(presented) && !ok {
			found, ok = t, true
		}
	}
	return found, ok
}

// check makes sure there is at least one token and that no two tokens share a
// name or a value.
func (ts ServiceTokens) check() error {
	if len(ts) == 0 {
		return errors.New("the service listener needs at least one service token")
	}
	names := make(map[string]bool, len(ts))
	hashes := make(map[[sha256.Size]byte]string, len(ts))
	for _, t := range ts {
		if t.Name == "" {
			return errors.New("service token: name must not be empty")
		}
		if names[t.Name] {
			return errors.Errorf("service tokens: duplicate name %q", t.Name)
		}
		names[t.Name] = true
		if other, dup := hashes[t.hash]; dup {
			return errors.Errorf("service tokens %q and %q have the same value", other, t.Name)
		}
		hashes[t.hash] = t.Name
	}
	return nil
}
