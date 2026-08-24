package lighthouse

import (
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

const (
	trustMarkStatusTestEntityID = "https://lighthouse.example.org"
	trustMarkStatusTestSubject  = "https://subject.example.org"
	trustMarkStatusTestType     = "test-trust-mark"
)

// testTrustMarkVersatileSigner is a minimal VersatileSigner that signs with a
// single key and exposes a JWKS containing the matching public key with the
// correct algorithm. This mirrors the production signer, whose JWKS keys carry
// the same alg as the signing header (unlike SingleKeyVersatileSigner, which
// hardcodes ES512 in its JWKS).
type testTrustMarkVersatileSigner struct {
	sk   jwx.SigningKey
	alg  jwa.SignatureAlgorithm
	jwks jwx.JWKS
}

func (s testTrustMarkVersatileSigner) Signer(algs ...string) (jwx.SigningKey, jwa.SignatureAlgorithm) {
	for _, a := range algs {
		if a == s.alg.String() {
			return s.sk, s.alg
		}
	}
	return nil, jwa.SignatureAlgorithm{}
}

func (s testTrustMarkVersatileSigner) DefaultSigner() (jwx.SigningKey, jwa.SignatureAlgorithm) {
	return s.sk, s.alg
}

func (s testTrustMarkVersatileSigner) JWKS() (jwx.JWKS, error) {
	return s.jwks, nil
}

// newTrustMarkStatusTestLightHouse builds a LightHouse with a real signer and
// TrustMarkIssuer wired up for determineTrustMarkStatus to work.
func newTrustMarkStatusTestLightHouse(t *testing.T) *LightHouse {
	t.Helper()
	sk, _, _, err := jwx.GenerateKeyPair(jwa.ES256(), 0)
	require.NoError(t, err)

	jwks := jwx.NewJWKS()
	pub, err := jwk.PublicKeyOf(sk)
	require.NoError(t, err)
	require.NoError(t, jwk.AssignKeyID(pub))
	require.NoError(t, pub.Set(jwk.AlgorithmKey, jwa.ES256()))
	require.NoError(t, jwks.AddKey(pub))

	versatile := testTrustMarkVersatileSigner{sk: sk, alg: jwa.ES256(), jwks: jwks}
	gs := jwx.NewGeneralJWTSigner(versatile, []jwa.SignatureAlgorithm{jwa.ES256()})

	fed := &LightHouse{
		FederationEntity: stubFedEntity{},
		TrustMarkIssuer:  oidfed.NewTrustMarkIssuer(trustMarkStatusTestEntityID, gs.TrustMarkSigner(), nil),
		GeneralJWTSigner: gs,
	}
	fed.TrustMarkIssuer.AddTrustMark(oidfed.TrustMarkSpec{TrustMarkType: trustMarkStatusTestType})
	return fed
}

func issueTrustMarkWithLifetime(t *testing.T, fed *LightHouse, lifetime time.Duration) string {
	t.Helper()
	tm, _, err := fed.IssueTrustMark(trustMarkStatusTestType, trustMarkStatusTestSubject, lifetime)
	require.NoError(t, err)
	return tm
}

// createTrustMarkSubject creates a TrustMarkSpec and a subject record for it in
// the passed storage, returning the subject ID. The issued trust mark instances
// have a foreign key to the subject, so a subject must exist for Create to work.
func createTrustMarkSubject(t *testing.T, store *storage.Storage) uint {
	t.Helper()
	specStore := store.TrustMarkSpecStorage()
	spec, err := specStore.Create(&model.AddTrustMarkSpec{TrustMarkType: trustMarkStatusTestType})
	require.NoError(t, err)
	subject, err := specStore.CreateSubject(
		spec.TrustMarkType,
		&model.AddTrustMarkSubject{EntityID: trustMarkStatusTestSubject, Status: model.StatusActive},
	)
	require.NoError(t, err)
	return subject.ID
}

// TestDetermineTrustMarkStatus_ExpiredValidMark is a regression test for the bug
// where a validly signed trust mark whose exp has passed was reported as
// "invalid" instead of "expired". jwt.Parse validates exp by default, so the
// expired mark previously failed at the signature-verification step and was
// classified as invalid before the expiration checks could run.
func TestDetermineTrustMarkStatus_ExpiredValidMark(t *testing.T) {
	fed := newTrustMarkStatusTestLightHouse(t)
	tm := issueTrustMarkWithLifetime(t, fed, time.Second)
	time.Sleep(1500 * time.Millisecond)

	status, err := fed.determineTrustMarkStatus(tm, TrustMarkStatusConfig{})
	require.NoError(t, err)
	assert.Equal(t, model.TrustMarkStatusExpired, status)
}

// TestDetermineTrustMarkStatus_Active verifies that a freshly issued, non-expired
// mark is reported as active.
func TestDetermineTrustMarkStatus_Active(t *testing.T) {
	fed := newTrustMarkStatusTestLightHouse(t)
	tm := issueTrustMarkWithLifetime(t, fed, time.Hour)

	status, err := fed.determineTrustMarkStatus(tm, TrustMarkStatusConfig{})
	require.NoError(t, err)
	assert.Equal(t, model.TrustMarkStatusActive, status)
}

// TestDetermineTrustMarkStatus_InvalidSignature verifies that a mark whose
// signature does not verify is reported as invalid.
func TestDetermineTrustMarkStatus_InvalidSignature(t *testing.T) {
	fed := newTrustMarkStatusTestLightHouse(t)
	tm := issueTrustMarkWithLifetime(t, fed, time.Hour)

	segments := strings.Split(tm, ".")
	require.Len(t, segments, 3)
	segments[2] = "AAAA" + segments[2][4:]
	tampered := strings.Join(segments, ".")

	status, err := fed.determineTrustMarkStatus(tampered, TrustMarkStatusConfig{})
	require.NoError(t, err)
	assert.Equal(t, model.TrustMarkStatusInvalid, status)
}

// TestDetermineTrustMarkStatus_ExpiredViaInstanceStore verifies that an expired
// mark is reported as expired when the instance store is configured and the
// stored instance has expired.
func TestDetermineTrustMarkStatus_ExpiredViaInstanceStore(t *testing.T) {
	fed := newTrustMarkStatusTestLightHouse(t)
	store := newTestStorage(t)
	backends, err := store.Backends(storage.JTIStorageDB)
	require.NoError(t, err)
	instanceStore := backends.TrustMarkInstances
	require.NotNil(t, instanceStore)

	jti := "test-expired-jti"
	tm, _, err := fed.IssueTrustMarkWithOptions(
		trustMarkStatusTestType, trustMarkStatusTestSubject, oidfed.IssueTrustMarkOptions{
			Lifetime:      time.Hour,
			SubjectClaims: map[string]any{"jti": jti},
		},
	)
	require.NoError(t, err)

	subjectID := createTrustMarkSubject(t, store)
	require.NoError(t, instanceStore.Create(&model.IssuedTrustMarkInstance{
		JTI:                jti,
		TrustMarkType:      trustMarkStatusTestType,
		Subject:            trustMarkStatusTestSubject,
		TrustMarkSubjectID: subjectID,
		ExpiresAt:          int(time.Now().Add(-time.Hour).Unix()),
		Revoked:            false,
	}))

	status, err := fed.determineTrustMarkStatus(tm, TrustMarkStatusConfig{InstanceStore: instanceStore})
	require.NoError(t, err)
	assert.Equal(t, model.TrustMarkStatusExpired, status)
}

// TestDetermineTrustMarkStatus_Revoked verifies that a revoked mark is reported
// as revoked when the instance store is configured.
func TestDetermineTrustMarkStatus_Revoked(t *testing.T) {
	fed := newTrustMarkStatusTestLightHouse(t)
	store := newTestStorage(t)
	backends, err := store.Backends(storage.JTIStorageDB)
	require.NoError(t, err)
	instanceStore := backends.TrustMarkInstances
	require.NotNil(t, instanceStore)

	jti := "test-revoked-jti"
	tm, _, err := fed.IssueTrustMarkWithOptions(
		trustMarkStatusTestType, trustMarkStatusTestSubject, oidfed.IssueTrustMarkOptions{
			Lifetime:      time.Hour,
			SubjectClaims: map[string]any{"jti": jti},
		},
	)
	require.NoError(t, err)

	subjectID := createTrustMarkSubject(t, store)
	require.NoError(t, instanceStore.Create(&model.IssuedTrustMarkInstance{
		JTI:                jti,
		TrustMarkType:      trustMarkStatusTestType,
		Subject:            trustMarkStatusTestSubject,
		TrustMarkSubjectID: subjectID,
		ExpiresAt:          int(time.Now().Add(time.Hour).Unix()),
		Revoked:            true,
	}))

	status, err := fed.determineTrustMarkStatus(tm, TrustMarkStatusConfig{InstanceStore: instanceStore})
	require.NoError(t, err)
	assert.Equal(t, model.TrustMarkStatusRevoked, status)
}
