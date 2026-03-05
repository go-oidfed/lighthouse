package storage

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// PublishedTrustMarksStorage provides CRUD access to PublishedTrustMark records
// implementing model.PublishedTrustMarksStore.
type PublishedTrustMarksStorage struct {
	db *gorm.DB
}

// List returns all published trust marks.
func (s *PublishedTrustMarksStorage) List() ([]model.PublishedTrustMark, error) {
	var items []model.PublishedTrustMark
	if err := s.db.Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "published_trust_marks: list failed")
	}
	return items, nil
}

// Create creates a new trust mark entry after validating the input.
func (s *PublishedTrustMarksStorage) Create(add model.AddTrustMark) (*model.PublishedTrustMark, error) {
	// Validate and normalize the input
	trustMarkType, trustMarkIssuer, err := s.validateAndExtractFromJWT(&add)
	if err != nil {
		return nil, err
	}

	item := &model.PublishedTrustMark{
		TrustMarkType:      trustMarkType,
		TrustMarkIssuer:    trustMarkIssuer,
		TrustMarkJWT:       add.TrustMark,
		Refresh:            add.Refresh,
		MinLifetime:        add.MinLifetime,
		RefreshGracePeriod: add.RefreshGracePeriod,
		RefreshRateLimit:   add.RefreshRateLimit,
		SelfIssuanceSpec:   add.SelfIssuanceSpec,
	}

	if err := s.db.Create(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark with this type already exists")
		}
		return nil, errors.Wrap(err, "published_trust_marks: create failed")
	}
	return item, nil
}

// findByIdent finds a trust mark by numeric ID or trust_mark_type.
func (s *PublishedTrustMarksStorage) findByIdent(ident string) (*model.PublishedTrustMark, error) {
	var item model.PublishedTrustMark

	// Try numeric ID first
	if id, err := strconv.ParseUint(ident, 10, 64); err == nil {
		if tx := s.db.First(&item, uint(id)); tx.Error == nil {
			return &item, nil
		}
	}

	// Fallback to trust_mark_type match
	if err := s.db.Where("trust_mark_type = ?", ident).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NotFoundError("trust mark not found")
		}
		return nil, errors.Wrap(err, "published_trust_marks: get failed")
	}
	return &item, nil
}

// Get retrieves a trust mark by ID or trust_mark_type.
func (s *PublishedTrustMarksStorage) Get(ident string) (*model.PublishedTrustMark, error) {
	return s.findByIdent(ident)
}

// Update replaces a trust mark entry entirely.
func (s *PublishedTrustMarksStorage) Update(ident string, update model.AddTrustMark) (*model.PublishedTrustMark, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}

	// Validate and normalize the input
	trustMarkType, trustMarkIssuer, err := s.validateAndExtractFromJWT(&update)
	if err != nil {
		return nil, err
	}

	// Check if trust_mark_type is changing and would conflict
	if trustMarkType != item.TrustMarkType {
		var existing model.PublishedTrustMark
		if err := s.db.Where("trust_mark_type = ? AND id != ?", trustMarkType, item.ID).First(&existing).Error; err == nil {
			return nil, model.AlreadyExistsError("trust mark with this type already exists")
		}
	}

	// Update all fields
	item.TrustMarkType = trustMarkType
	item.TrustMarkIssuer = trustMarkIssuer
	item.TrustMarkJWT = update.TrustMark
	item.Refresh = update.Refresh
	item.MinLifetime = update.MinLifetime
	item.RefreshGracePeriod = update.RefreshGracePeriod
	item.RefreshRateLimit = update.RefreshRateLimit
	item.SelfIssuanceSpec = update.SelfIssuanceSpec

	if err = s.db.Save(item).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, model.AlreadyExistsError("trust mark with this type already exists")
		}
		return nil, errors.Wrap(err, "published_trust_marks: update failed")
	}
	return item, nil
}

// Patch partially updates a trust mark entry (only non-nil fields).
func (s *PublishedTrustMarksStorage) Patch(ident string, patch model.UpdateTrustMark) (*model.PublishedTrustMark, error) {
	item, err := s.findByIdent(ident)
	if err != nil {
		return nil, err
	}

	// Apply only non-nil fields
	if patch.TrustMarkIssuer != nil {
		item.TrustMarkIssuer = *patch.TrustMarkIssuer
	}
	if patch.TrustMark != nil {
		item.TrustMarkJWT = *patch.TrustMark
		// If updating the JWT, extract and validate type/issuer
		if *patch.TrustMark != "" {
			claims, err := parseJWTClaims(*patch.TrustMark)
			if err == nil {
				// Validate that the JWT's trust mark type matches the existing record
				if jwtType, ok := claims["id"].(string); ok && jwtType != "" && jwtType != item.TrustMarkType {
					return nil, model.ValidationError("JWT trust mark type does not match existing record")
				}
				// Update issuer from JWT if not explicitly provided
				if patch.TrustMarkIssuer == nil {
					if jwtIssuer, ok := claims["iss"].(string); ok && jwtIssuer != "" {
						item.TrustMarkIssuer = jwtIssuer
					}
				}
			}
		}
	}
	if patch.Refresh != nil {
		item.Refresh = *patch.Refresh
	}
	if patch.MinLifetime != nil {
		item.MinLifetime = *patch.MinLifetime
	}
	if patch.RefreshGracePeriod != nil {
		item.RefreshGracePeriod = *patch.RefreshGracePeriod
	}
	if patch.RefreshRateLimit != nil {
		item.RefreshRateLimit = *patch.RefreshRateLimit
	}
	if patch.SelfIssuanceSpec != nil {
		item.SelfIssuanceSpec = patch.SelfIssuanceSpec
	}

	if err = s.db.Save(item).Error; err != nil {
		return nil, errors.Wrap(err, "published_trust_marks: patch failed")
	}
	return item, nil
}

// Delete removes a trust mark entry.
func (s *PublishedTrustMarksStorage) Delete(ident string) error {
	item, err := s.findByIdent(ident)
	if err != nil {
		return err
	}
	if err = s.db.Delete(item).Error; err != nil {
		return errors.Wrap(err, "published_trust_marks: delete failed")
	}
	return nil
}

// validateAndExtractFromJWT validates the input and extracts trust_mark_type and issuer from JWT if needed.
// Returns the resolved trust_mark_type and trust_mark_issuer.
func (*PublishedTrustMarksStorage) validateAndExtractFromJWT(add *model.AddTrustMark) (trustMarkType, trustMarkIssuer string, err error) {
	hasTrustMarkIssuer := add.TrustMarkIssuer != ""
	hasTrustMarkJWT := add.TrustMark != ""
	hasSelfIssuance := add.SelfIssuanceSpec != nil

	// Count how many creation modes are being used
	modeCount := 0
	if hasTrustMarkIssuer && !hasTrustMarkJWT && !hasSelfIssuance {
		modeCount++ // Mode 1: external fetching
	}
	if hasTrustMarkJWT {
		modeCount++ // Mode 2: direct JWT
	}
	if hasSelfIssuance {
		modeCount++ // Mode 3: self-issuance
	}

	if modeCount == 0 {
		return "", "", model.ValidationError("must provide one of: (trust_mark_type + trust_mark_issuer), trust_mark JWT, or self_issuance_spec")
	}
	if modeCount > 1 {
		// Allow JWT + trust_mark_type (type can be provided or extracted)
		// But don't allow conflicting modes
		if hasSelfIssuance && (hasTrustMarkJWT || hasTrustMarkIssuer) {
			return "", "", model.ValidationError("self_issuance_spec cannot be combined with trust_mark JWT or trust_mark_issuer")
		}
	}

	trustMarkType = add.TrustMarkType
	trustMarkIssuer = add.TrustMarkIssuer

	// If JWT is provided, parse it to extract/validate type and issuer
	if hasTrustMarkJWT {
		claims, parseErr := parseJWTClaims(add.TrustMark)
		if parseErr != nil {
			return "", "", model.ValidationError("invalid trust_mark JWT: " + parseErr.Error())
		}

		// Extract trust mark type from JWT "id" claim
		if jwtType, ok := claims["id"].(string); ok && jwtType != "" {
			if trustMarkType == "" {
				trustMarkType = jwtType
			} else if trustMarkType != jwtType {
				return "", "", model.ValidationError("trust_mark_type does not match JWT 'id' claim")
			}
		}

		// Extract issuer from JWT "iss" claim
		if jwtIssuer, ok := claims["iss"].(string); ok && jwtIssuer != "" {
			if trustMarkIssuer == "" {
				trustMarkIssuer = jwtIssuer
			} else if trustMarkIssuer != jwtIssuer {
				return "", "", model.ValidationError("trust_mark_issuer does not match JWT 'iss' claim")
			}
		}
	}

	// Self-issuance mode requires trust_mark_type
	if hasSelfIssuance && trustMarkType == "" {
		return "", "", model.ValidationError("trust_mark_type is required for self-issued trust marks")
	}

	// External fetching mode (no JWT, no self-issuance) requires both type and issuer
	if !hasTrustMarkJWT && !hasSelfIssuance {
		if trustMarkType == "" {
			return "", "", model.ValidationError("trust_mark_type is required")
		}
		if trustMarkIssuer == "" {
			return "", "", model.ValidationError("trust_mark_issuer is required for external trust mark fetching")
		}
	}

	// Final validation: trust_mark_type must be present
	if trustMarkType == "" {
		return "", "", model.ValidationError("could not determine trust_mark_type")
	}

	return trustMarkType, trustMarkIssuer, nil
}

// parseJWTClaims parses a JWT and returns the payload claims without verifying the signature.
// This is used to extract trust_mark_type (id) and issuer (iss) from the JWT.
func parseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format: expected 3 parts")
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode JWT payload")
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.Wrap(err, "failed to parse JWT claims")
	}

	return claims, nil
}
