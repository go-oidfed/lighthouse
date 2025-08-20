package lighthouse

import (
	"fmt"
	"slices"

	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/apimodel"
	"github.com/go-oidfed/lib/jwx"
)

// EntityChecker is an interface used to check if an entity satisfies
// some requirements, e.g. to check if an entity should be
// enrolled in the federation or should be issued a trust mark
type EntityChecker interface {
	// Check checks if the entity with the passed oidfed.EntityStatement
	// satisfies the requirements of this EntityChecker or not
	// It returns a bool indicating this status,
	// and if not a http status code as well as a oidfed.Error as api response
	Check(
		entityConfiguration *oidfed.EntityStatement,
		entityTypes []string,
	) (bool, int, *oidfed.Error)
	// Unmarshaler is used to load the configuration
	yaml.Unmarshaler
}

var entityCheckerRegistry = make(map[string]func() EntityChecker)

// RegisterEntityChecker registers a custom EntityChecker so
// EntityCheckerFromYAMLConfig knows about it and can return it from a yaml
// config
func RegisterEntityChecker(configTypeName string, constructor func() EntityChecker) {
	entityCheckerRegistry[configTypeName] = constructor
}

func init() {
	RegisterEntityChecker("none", func() EntityChecker { return &EntityCheckerNone{} })
	RegisterEntityChecker("trust_mark", func() EntityChecker { return &TrustMarkEntityChecker{} })
	RegisterEntityChecker("trust_path", func() EntityChecker { return &TrustPathEntityChecker{} })
	RegisterEntityChecker("authority_hints", func() EntityChecker { return &AuthorityHintEntityChecker{} })
	RegisterEntityChecker("entity_id", func() EntityChecker { return &EntityIDEntityChecker{} })
	RegisterEntityChecker("multiple_and", func() EntityChecker { return &MultipleEntityCheckerAnd{} })
	RegisterEntityChecker("multiple_or", func() EntityChecker { return &MultipleEntityCheckerOr{} })
}

// EntityCheckerConfig is a type for configuring an EntityChecker through yaml
type EntityCheckerConfig struct {
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config,omitempty"`
}

// EntityCheckerFromYAMLConfig passes the passed yaml config and returns the
// configured EntityChecker
func EntityCheckerFromYAMLConfig(config []byte) (EntityChecker, error) {
	var c EntityCheckerConfig
	if err := yaml.Unmarshal(config, &c); err != nil {
		return nil, errors.WithStack(err)
	}
	return EntityCheckerFromEntityCheckerConfig(c)
}

// EntityCheckerFromEntityCheckerConfig parses the passed EntityCheckerConfig
// and returns the configured EntityChecker
func EntityCheckerFromEntityCheckerConfig(c EntityCheckerConfig) (
	EntityChecker,
	error,
) {
	checkerConstructor := entityCheckerRegistry[c.Type]
	if checkerConstructor == nil {
		return nil, errors.Errorf("unknown entity check type: %s", c.Type)
	}
	checker := checkerConstructor()
	if err := checker.UnmarshalYAML(&c.Config); err != nil {
		return nil, errors.WithStack(err)
	}
	return checker, nil
}

// EntityCheckerNone is a type implementing EntityChecker but that checks
// nothing
type EntityCheckerNone struct{}

// Check implements the EntityChecker interface
func (EntityCheckerNone) Check(_ *oidfed.EntityStatement, _ []string) (
	bool, int, *oidfed.Error,
) {
	return true, 0, nil
}

// UnmarshalYAML implements the EntityChecker interface
func (EntityCheckerNone) UnmarshalYAML(_ *yaml.Node) error {
	return nil
}

// MultipleEntityCheckerOr is an EntityChecker that combines multiple
// EntityChecker by requiring only one check to pass
type MultipleEntityCheckerOr struct {
	Checkers []EntityChecker
}

// NewMultipleEntityCheckerOr returns a new MultipleEntityCheckerOr using
// all the passed EntityChecker
func NewMultipleEntityCheckerOr(checkers ...EntityChecker) *MultipleEntityCheckerOr {
	return &MultipleEntityCheckerOr{Checkers: checkers}
}

// Check implements the EntityChecker interface
func (c MultipleEntityCheckerOr) Check(
	entityStatement *oidfed.EntityStatement, entityTypes []string,
) (bool, int, *oidfed.Error) {
	for _, checker := range c.Checkers {
		if ok, _, _ := checker.Check(entityStatement, entityTypes); ok {
			return true, 0, nil
		}
	}
	return false, fiber.StatusForbidden, &oidfed.Error{
		Error:            "forbidden",
		ErrorDescription: "no enrollment check passed",
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interfaces
func (c *MultipleEntityCheckerOr) UnmarshalYAML(node *yaml.Node) error {
	var datas []EntityCheckerConfig
	if err := node.Decode(&datas); err != nil {
		return errors.WithStack(err)
	}
	for _, data := range datas {
		checker, err := EntityCheckerFromEntityCheckerConfig(data)
		if err != nil {
			return errors.WithStack(err)
		}
		c.Checkers = append(c.Checkers, checker)
	}
	return nil
}

// MultipleEntityCheckerAnd is an EntityChecker that combines multiple
// EntityChecker by requiring all checks to pass
type MultipleEntityCheckerAnd struct {
	Checkers []EntityChecker
}

// NewMultipleEntityCheckerAnd returns a new MultipleEntityCheckerAnd using
// all the passed EntityChecker
func NewMultipleEntityCheckerAnd(
	checkers ...EntityChecker,
) *MultipleEntityCheckerAnd {
	return &MultipleEntityCheckerAnd{Checkers: checkers}
}

// Check implements the EntityChecker interface
func (c MultipleEntityCheckerAnd) Check(entityStatement *oidfed.EntityStatement, entityTypes []string) (
	bool, int, *oidfed.Error,
) {
	for _, checker := range c.Checkers {
		if ok, status, err := checker.Check(entityStatement, entityTypes); !ok {
			return ok, status, err
		}
	}
	return true, 0, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interfaces
func (c *MultipleEntityCheckerAnd) UnmarshalYAML(node *yaml.Node) error {
	var datas []EntityCheckerConfig
	if err := node.Decode(&datas); err != nil {
		return errors.WithStack(err)
	}
	for _, data := range datas {
		checker, err := EntityCheckerFromEntityCheckerConfig(data)
		if err != nil {
			return errors.WithStack(err)
		}
		c.Checkers = append(c.Checkers, checker)
	}
	return nil
}

// TrustMarkEntityChecker checks that the entity has a
// valid trust mark. The trust mark can be checked with a specific issuer or
// through the federation
type TrustMarkEntityChecker struct {
	TrustMarkType       string                    `yaml:"trust_mark_type"`
	TrustAnchors        oidfed.TrustAnchors       `yaml:"trust_anchors"`
	TrustMarkIssuerJWKS jwx.JWKS                  `yaml:"trust_mark_issuer_jwks"`
	TrustMarkOwnerSpec  oidfed.TrustMarkOwnerSpec `yaml:"trust_mark_owner"`
}

// Check implements the EntityChecker interface
func (c TrustMarkEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	entityTypes []string, // skipcq: RVV-B0012
) (bool, int, *oidfed.Error) {
	tms := entityConfiguration.TrustMarks
	noTrustMarkError := &oidfed.Error{
		Error:            "forbidden",
		ErrorDescription: fmt.Sprintf("entity does not contain required trust mark '%s'", c.TrustMarkType),
	}
	if len(tms) == 0 {
		return false, fiber.StatusForbidden, noTrustMarkError
	}
	tm := tms.FindByID(c.TrustMarkType)
	if tm == nil {
		return false, fiber.StatusForbidden, noTrustMarkError
	}
	if c.TrustMarkIssuerJWKS.Set != nil && c.TrustMarkIssuerJWKS.Len() != 0 {
		if err := tm.VerifyExternal(
			c.TrustMarkIssuerJWKS,
			c.TrustMarkOwnerSpec,
		); err == nil {
			return true, 0, nil
		}
	} else {
		for _, ta := range c.TrustAnchors {
			taConfig, err := oidfed.GetEntityConfiguration(ta.EntityID)
			if err != nil {
				continue
			}
			if err = tm.VerifyFederation(
				&taConfig.
					EntityStatementPayload,
			); err != nil {
				return true, 0, nil
			}
		}
	}
	return false, fiber.StatusForbidden, &oidfed.Error{
		Error: "forbidden",
		ErrorDescription: fmt.Sprintf(
			"could not verify required trust mark '%s'", c.TrustMarkType,
		),
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interface
func (c *TrustMarkEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	type Alias TrustMarkEntityChecker
	alias := Alias(*c)
	err := node.Decode(&alias)
	if err != nil {
		return err
	}
	*c = TrustMarkEntityChecker(alias)
	return nil
}

// TrustPathEntityChecker checks that the entity has a
// valid trust path to a trust anchor
type TrustPathEntityChecker struct {
	TrustAnchors                oidfed.TrustAnchors `yaml:"trust_anchors"`
	isAlreadyTrustAnchorChecker EntityIDEntityChecker
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interface
func (c *TrustPathEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	type Alias TrustPathEntityChecker
	alias := Alias(*c)
	err := node.Decode(&alias)
	if err != nil {
		return err
	}
	*c = TrustPathEntityChecker(alias)
	c.isAlreadyTrustAnchorChecker = EntityIDEntityChecker{AllowedIDs: c.TrustAnchors.EntityIDs()}
	return nil
}

// Check implements the EntityChecker interface
func (c TrustPathEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	entityTypes []string,
) (bool, int, *oidfed.Error) {
	if ok, _, _ := c.isAlreadyTrustAnchorChecker.Check(entityConfiguration, entityTypes); ok {
		return true, 0, nil
	}

	confirmedValid, _ := oidfed.DefaultMetadataResolver.ResolvePossible(
		apimodel.ResolveRequest{
			Subject:     entityConfiguration.Subject,
			TrustAnchor: c.TrustAnchors.EntityIDs(),
		},
	)
	if !confirmedValid {
		return false, fiber.StatusForbidden, &oidfed.Error{
			Error:            "forbidden",
			ErrorDescription: "no valid trust path to trust anchors found",
		}
	}
	return true, 0, nil
}

// EntityIDEntityChecker checks that the entity has a
// certain entity id
type EntityIDEntityChecker struct {
	AllowedIDs []string `yaml:"entity_ids"`
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interface
func (c *EntityIDEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	type Alias EntityIDEntityChecker
	alias := Alias(*c)
	err := node.Decode(&alias)
	if err != nil {
		return err
	}
	*c = EntityIDEntityChecker(alias)
	return nil
}

// Check implements the EntityChecker interface
func (c EntityIDEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	_ []string,
) (bool, int, *oidfed.Error) {
	if !slices.Contains(c.AllowedIDs, entityConfiguration.Subject) {
		return false, fiber.StatusBadRequest, oidfed.ErrorInvalidRequest("this entity is not allowed")
	}
	return true, 0, nil
}

// AuthorityHintEntityChecker checks that the entity has a
// certain entry in its authority_hints
type AuthorityHintEntityChecker struct {
	EntityID string `yaml:"entity_id"`
}

// UnmarshalYAML implements the yaml.Unmarshaler and EntityChecker interface
func (c *AuthorityHintEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	type Alias AuthorityHintEntityChecker
	alias := Alias(*c)
	err := node.Decode(&alias)
	if err != nil {
		return err
	}
	*c = AuthorityHintEntityChecker(alias)
	return nil
}

// Check implements the EntityChecker interface
func (c AuthorityHintEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	_ []string,
) (bool, int, *oidfed.Error) {
	if !slices.Contains(entityConfiguration.AuthorityHints, c.EntityID) {
		return false, fiber.StatusBadRequest, oidfed.ErrorInvalidRequest(
			fmt.Sprintf("must include '%s' in authority_hints", c.EntityID),
		)
	}
	return true, 0, nil
}
