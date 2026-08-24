package lighthouse

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"

	"github.com/go-oidfed/lighthouse/storage/model"
)

const (
	trustMarkIssuanceTestEntityID = "https://lighthouse.example.org"
	trustMarkIssuanceTestSubject  = "https://subject.example.org"
	trustMarkIssuanceTestType     = "test-trust-mark"
)

// trustMarkIssuanceSpecStore is a minimal model.TrustMarkSpecStore stub that
// serves a single spec and optionally a single subject with custom claims.
type trustMarkIssuanceSpecStore struct {
	model.TrustMarkSpecStore
	spec    *model.TrustMarkSpec
	subject *model.TrustMarkSubject
}

func (s *trustMarkIssuanceSpecStore) GetByType(trustMarkType string) (*model.TrustMarkSpec, error) {
	if s.spec == nil || s.spec.TrustMarkType != trustMarkType {
		return nil, errors.New("spec not found")
	}
	return s.spec, nil
}

func (s *trustMarkIssuanceSpecStore) GetSubject(specIdent, subjectIdent string) (*model.TrustMarkSubject, error) {
	if s.subject == nil || s.subject.EntityID != subjectIdent {
		return nil, errors.New("subject not found")
	}
	return s.subject, nil
}

// trustMarkIssuanceEligibilityStore is a minimal
// model.TrustMarkedEntitiesStorageBackend stub that reports the configured
// status for every subject.
type trustMarkIssuanceEligibilityStore struct {
	model.TrustMarkedEntitiesStorageBackend
	status model.Status
}

func (s trustMarkIssuanceEligibilityStore) TrustMarkedStatus(trustMarkType, entityID string) (model.Status, error) {
	return s.status, nil
}

// newTrustMarkIssuanceTestLightHouse builds a LightHouse with a real signer,
// a DB-backed trust mark spec provider and the given spec store wired up, so
// that the trust mark endpoint resolves specs and eligibility from storage.
func newTrustMarkIssuanceTestLightHouse(t *testing.T, specStore model.TrustMarkSpecStore) *LightHouse {
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
		TrustMarkIssuer:  oidfed.NewTrustMarkIssuer(trustMarkIssuanceTestEntityID, gs.TrustMarkSigner(), nil),
		GeneralJWTSigner: gs,
		fedMetadata:      oidfed.FederationEntityMetadata{},
	}
	fed.TrustMarkIssuer.SetProvider(NewDBTrustMarkSpecProvider(specStore))
	return fed
}

// setupTrustMarkIssuanceApp registers the trust mark endpoint on a fiber app
// routed through the lighthouse dispatcher.
func setupTrustMarkIssuanceApp(t *testing.T, fed *LightHouse, config TrustMarkEndpointConfig) *fiber.App {
	t.Helper()
	require.NoError(t, fed.AddTrustMarkEndpointWithConfig(
		EndpointConf{Path: "/tm"},
		config,
	))
	app := fiber.New()
	app.All("/*", fed.dispatch)
	return app
}

// requestTrustMark issues a GET request for a trust mark and returns the signed
// JWT on success.
func requestTrustMark(t *testing.T, app *fiber.App, trustMarkType, sub string) string {
	t.Helper()
	req := httptest.NewRequest(
		fiber.MethodGet,
		"/tm?sub="+url.QueryEscape(sub)+"&trust_mark_type="+url.QueryEscape(trustMarkType),
		nil,
	)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return string(body)
}

// issueAndParseTrustMark requests a trust mark and parses it into a
// oidfed.TrustMark.
func issueAndParseTrustMark(t *testing.T, app *fiber.App, trustMarkType, sub string) *oidfed.TrustMark {
	t.Helper()
	tm, err := oidfed.ParseTrustMark([]byte(requestTrustMark(t, app, trustMarkType, sub)))
	require.NoError(t, err)
	return tm
}

// trustMarkIssuanceTestSpec returns a spec with general additional claims and
// optionally a subject with its own (custom) additional claims.
func trustMarkIssuanceTestSpec(subjectClaims map[string]any) (*model.TrustMarkSpec, *model.TrustMarkSubject) {
	spec := &model.TrustMarkSpec{
		TrustMarkType:    trustMarkIssuanceTestType,
		AdditionalClaims: map[string]any{"general": "g", "shared": "spec"},
	}
	var subject *model.TrustMarkSubject
	if subjectClaims != nil {
		subject = &model.TrustMarkSubject{
			EntityID:         trustMarkIssuanceTestSubject,
			Status:           model.StatusActive,
			AdditionalClaims: subjectClaims,
		}
	}
	return spec, subject
}

// TestTrustMarkIssuance_SubjectClaimsOverrideSpecClaims verifies that when a
// subject has custom additional claims they are used exclusively and the spec's
// general additional claims are not included.
func TestTrustMarkIssuance_SubjectClaimsOverrideSpecClaims(t *testing.T) {
	spec, subject := trustMarkIssuanceTestSpec(map[string]any{"custom": "c", "shared": "subject"})
	specStore := &trustMarkIssuanceSpecStore{spec: spec, subject: subject}

	fed := newTrustMarkIssuanceTestLightHouse(t, specStore)
	app := setupTrustMarkIssuanceApp(t, fed, TrustMarkEndpointConfig{
		Store:     trustMarkIssuanceEligibilityStore{status: model.StatusActive},
		SpecStore: specStore,
	})

	tm := issueAndParseTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject)

	assert.Equal(t, "c", tm.Extra["custom"], "subject claims should be present")
	assert.Equal(t, "subject", tm.Extra["shared"], "subject claims should win over spec claims")
	assert.NotContains(t, tm.Extra, "general", "spec's general claims must not be merged in")
	assert.NotEmpty(t, tm.Extra["jti"], "jti should always be present")
}

// TestTrustMarkIssuance_SpecClaimsWhenNoSubjectClaims verifies that when the
// subject has no custom claims, the spec's general additional claims are used.
func TestTrustMarkIssuance_SpecClaimsWhenNoSubjectClaims(t *testing.T) {
	spec, subject := trustMarkIssuanceTestSpec(nil)
	specStore := &trustMarkIssuanceSpecStore{spec: spec, subject: subject}

	fed := newTrustMarkIssuanceTestLightHouse(t, specStore)
	app := setupTrustMarkIssuanceApp(t, fed, TrustMarkEndpointConfig{
		Store:     trustMarkIssuanceEligibilityStore{status: model.StatusActive},
		SpecStore: specStore,
	})

	tm := issueAndParseTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject)

	assert.Equal(t, "g", tm.Extra["general"], "spec's general claims should be present")
	assert.Equal(t, "spec", tm.Extra["shared"], "spec's general claims should be present")
	assert.NotEmpty(t, tm.Extra["jti"], "jti should always be present")
}

// TestTrustMarkIssuance_NoSpecClaims verifies that when neither the subject nor
// the spec defines additional claims, only the jti is present.
func TestTrustMarkIssuance_NoSpecClaims(t *testing.T) {
	spec, subject := trustMarkIssuanceTestSpec(nil)
	spec.AdditionalClaims = nil
	specStore := &trustMarkIssuanceSpecStore{spec: spec, subject: subject}

	fed := newTrustMarkIssuanceTestLightHouse(t, specStore)
	app := setupTrustMarkIssuanceApp(t, fed, TrustMarkEndpointConfig{
		Store:     trustMarkIssuanceEligibilityStore{status: model.StatusActive},
		SpecStore: specStore,
	})

	tm := issueAndParseTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject)

	assert.NotEmpty(t, tm.Extra["jti"], "jti should always be present")
	assert.Equal(t, 1, len(tm.Extra), "only jti expected")
}

// TestTrustMarkIssuance_NoSubjectRecordUsesSpecClaims verifies that a subject
// without a record in storage falls back to the spec's general claims.
func TestTrustMarkIssuance_NoSubjectRecordUsesSpecClaims(t *testing.T) {
	spec, _ := trustMarkIssuanceTestSpec(nil)
	specStore := &trustMarkIssuanceSpecStore{spec: spec}

	fed := newTrustMarkIssuanceTestLightHouse(t, specStore)
	app := setupTrustMarkIssuanceApp(t, fed, TrustMarkEndpointConfig{
		Store:     trustMarkIssuanceEligibilityStore{status: model.StatusActive},
		SpecStore: specStore,
	})

	tm := issueAndParseTrustMark(t, app, trustMarkIssuanceTestType, trustMarkIssuanceTestSubject)

	assert.Equal(t, "g", tm.Extra["general"], "spec's general claims should be present")
	assert.NotEmpty(t, tm.Extra["jti"], "jti should always be present")
}
