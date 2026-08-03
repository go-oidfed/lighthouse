package main

import (
	"encoding/json"
	"fmt"

	"github.com/go-oidfed/lighthouse/internal/migration"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// migrateTrustAnchors migrates trust anchors from the config file to the database.
// It collects trust anchors from federation_data.trust_anchors,
// endpoints.auth.trust_anchors, per-endpoint auth_trust_anchors, and checker
// trust_anchors, deduplicating by entity_id.
func (m *configMigrator) migrateTrustAnchors() []migrationResult {
	var results []migrationResult

	type taEntry struct {
		conf     migration.TrustAnchorConf
		location string
	}
	allTAs := make(map[string]*taEntry)

	// 1. federation_data.trust_anchors
	for _, ta := range m.config.Federation.TrustAnchors {
		if ta.EntityID == "" {
			continue
		}
		if _, exists := allTAs[ta.EntityID]; !exists {
			allTAs[ta.EntityID] = &taEntry{conf: ta, location: "federation_data.trust_anchors"}
		}
	}

	// 2. endpoints.auth.trust_anchors
	for _, ta := range m.config.Endpoints.Auth.TrustAnchors {
		if ta.EntityID == "" {
			continue
		}
		if _, exists := allTAs[ta.EntityID]; !exists {
			allTAs[ta.EntityID] = &taEntry{conf: ta, location: "endpoints.auth.trust_anchors"}
		}
	}

	// 3. Per-endpoint auth_trust_anchors (entity IDs only, no JWKS)
	collectAuthTAs := func(authTAs []string, location string) {
		for _, entityID := range authTAs {
			if entityID == "" {
				continue
			}
			if _, exists := allTAs[entityID]; !exists {
				allTAs[entityID] = &taEntry{
					conf:     migration.TrustAnchorConf{EntityID: entityID},
					location: location,
				}
			}
		}
	}
	collectAuthTAs(m.config.Endpoints.Fetch.AuthTrustAnchors, "endpoints.fetch.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.List.AuthTrustAnchors, "endpoints.list.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.Resolve.AuthTrustAnchors, "endpoints.resolve.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.TrustMarkStatus.AuthTrustAnchors, "endpoints.trust_mark_status.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.TrustMarkList.AuthTrustAnchors, "endpoints.trust_mark_list.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.TrustMark.AuthTrustAnchors, "endpoints.trust_mark.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.HistoricalKeys.AuthTrustAnchors, "endpoints.historical_keys.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.Enroll.AuthTrustAnchors, "endpoints.enroll.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.EnrollRequest.AuthTrustAnchors, "endpoints.enroll_request.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.TrustMarkRequest.AuthTrustAnchors, "endpoints.trust_mark_request.auth_trust_anchors")
	collectAuthTAs(m.config.Endpoints.EntityCollection.AuthTrustAnchors, "endpoints.entity_collection.auth_trust_anchors")

	// 4. Allowed trust anchors (entity IDs only)
	collectAuthTAs(m.config.Endpoints.Resolve.AllowedTrustAnchors, "endpoints.resolve.allowed_trust_anchors")
	collectAuthTAs(m.config.Endpoints.EntityCollection.AllowedTrustAnchors, "endpoints.entity_collection.allowed_trust_anchors")

	// 5. Checker trust_anchors (entity IDs only)
	if m.config.Endpoints.Enroll.Checker.Kind != 0 {
		taIDs := migration.ExtractCheckerTrustAnchorIDs(&m.config.Endpoints.Enroll.Checker)
		collectAuthTAs(taIDs, "endpoints.enroll.checker.trust_anchors")
	}

	if len(allTAs) == 0 {
		results = append(results, migrationResult{
			section: sectionTrustAnchors,
			action:  "skipped",
			details: "no trust anchors found in config",
		})
		return results
	}

	existingList, err := m.backends.TrustAnchors.List()
	if err != nil {
		results = append(results, migrationResult{
			section: sectionTrustAnchors,
			err:     fmt.Errorf("failed to list existing trust anchors: %w", err),
		})
		return results
	}
	existingMap := make(map[string]bool)
	for _, ta := range existingList {
		existingMap[ta.EntityID] = true
	}

	for entityID, entry := range allTAs {
		result := migrationResult{
			section: sectionTrustAnchors,
			details: entityID,
		}

		if existingMap[entityID] && !m.force {
			result.action = "skipped"
			result.details = fmt.Sprintf("%s already exists", entityID)
			results = append(results, result)
			continue
		}

		if m.dryRun {
			result.action = "dry-run"
			result.details = fmt.Sprintf("would add %s (from %s)", entityID, entry.location)
			results = append(results, result)
			continue
		}

		var jwks *model.JWKS
		if entry.conf.JWKSFile != "" {
			jwks, err = migration.ParseJWKSFile(entry.conf.JWKSFile)
			if err != nil {
				result.err = fmt.Errorf("failed to parse jwks_file %q: %w", entry.conf.JWKSFile, err)
				results = append(results, result)
				continue
			}
		} else if entry.conf.JWKS != nil {
			jwks, err = migration.ParseJWKSFromAny(entry.conf.JWKS)
			if err != nil {
				result.err = fmt.Errorf("failed to parse inline jwks: %w", err)
				results = append(results, result)
				continue
			}
		}

		req := model.AddTrustAnchor{
			EntityID:         entityID,
			JWKS:             jwks,
			EnableJWKSUpdate: entry.conf.EnableJWKSUpdate,
			KeyPollInterval:  int64(entry.conf.KeyPollInterval.Duration().Seconds()),
		}

		if existingMap[entityID] {
			_, err = m.backends.TrustAnchors.Update(entityID, req)
		} else {
			_, err = m.backends.TrustAnchors.Create(req)
		}
		if err != nil {
			result.err = err
		} else {
			result.action = "created"
			if existingMap[entityID] {
				result.action = "overwritten"
			}
		}
		results = append(results, result)
	}

	return results
}

// migrateEndpoints migrates federation endpoint configurations from the config
// file to the database.
func (m *configMigrator) migrateEndpoints() []migrationResult {
	var results []migrationResult

	allRequireAuth := m.config.Endpoints.Auth.AllRequireAuth
	defaultTAEntityIDs := make([]string, 0, len(m.config.Endpoints.Auth.TrustAnchors))
	for _, ta := range m.config.Endpoints.Auth.TrustAnchors {
		defaultTAEntityIDs = append(defaultTAEntityIDs, ta.EntityID)
	}

	// createOrUpdateEndpoint creates or updates an endpoint in the DB.
	createOrUpdateEndpoint := func(req model.AddFederationEndpoint) migrationResult {
		result := migrationResult{section: sectionEndpoints, details: string(req.Type)}
		existing, _ := m.backends.FederationEndpoints.GetByType(req.Type)
		if existing != nil && !m.force {
			result.action = "skipped"
			result.details = fmt.Sprintf("%s already exists", req.Type)
			return result
		}
		if m.dryRun {
			result.action = "dry-run"
			path := ""
			if req.Path != nil {
				path = *req.Path
			}
			result.details = fmt.Sprintf("would add %s at %s", req.Type, path)
			return result
		}
		if existing != nil {
			if _, err := m.backends.FederationEndpoints.Update(req.Type, req); err != nil {
				result.err = err
			} else {
				result.action = "overwritten"
			}
		} else {
			if _, err := m.backends.FederationEndpoints.Create(req); err != nil {
				result.err = err
			} else {
				result.action = "created"
			}
		}
		return result
	}

	// buildSimpleReq builds an AddFederationEndpoint for a simple endpoint.
	buildSimpleReq := func(
		epType model.FederationEndpointType, path, url string,
		authEnabled bool, authTAs []string,
	) model.AddFederationEndpoint {
		return model.AddFederationEndpoint{
			Type:             epType,
			Path:             migration.StrPtrOrNil(path),
			URL:              migration.StrPtrOrNil(url),
			AuthEnabled:      authEnabled,
			AuthTrustAnchors: authTAs,
		}
	}

	// Helper for simple endpoints (path/url/auth only).
	migrateSimple := func(
		epType model.FederationEndpointType, path, url string,
		authEnabled bool, authTAs []string,
	) {
		if path == "" && url == "" {
			return
		}
		if authEnabled && len(authTAs) == 0 {
			authTAs = defaultTAEntityIDs
		}
		results = append(results, createOrUpdateEndpoint(buildSimpleReq(epType, path, url, authEnabled, authTAs)))
	}

	// Fetch
	migrateSimple(model.EndpointTypeFetch,
		m.config.Endpoints.Fetch.Path, m.config.Endpoints.Fetch.URL,
		m.config.Endpoints.Fetch.AuthEnabled || allRequireAuth,
		m.config.Endpoints.Fetch.AuthTrustAnchors)

	// List
	migrateSimple(model.EndpointTypeList,
		m.config.Endpoints.List.Path, m.config.Endpoints.List.URL,
		m.config.Endpoints.List.AuthEnabled || allRequireAuth,
		m.config.Endpoints.List.AuthTrustAnchors)

	// Resolve
	if m.config.Endpoints.Resolve.Path != "" || m.config.Endpoints.Resolve.URL != "" {
		authTAs := m.config.Endpoints.Resolve.AuthTrustAnchors
		authEnabled := m.config.Endpoints.Resolve.AuthEnabled || allRequireAuth
		if authEnabled && len(authTAs) == 0 {
			authTAs = defaultTAEntityIDs
		}
		cfg := migration.ResolveDBConfigJSON{
			AllowedTrustAnchors:                    m.config.Endpoints.Resolve.AllowedTrustAnchors,
			UseEntityCollectionAllowedTrustAnchors: m.config.Endpoints.Resolve.UseEntityCollectionAllowedTrustAnchors,
			GracePeriodSeconds:                     int64(m.config.Endpoints.Resolve.GracePeriod.Duration().Seconds()),
			TimeElapsedGraceFactor:                 m.config.Endpoints.Resolve.TimeElapsedGraceFactor,
			ProactiveResolver: &migration.ProactiveResolverDBConfigJSON{
				Enabled:                  m.config.Endpoints.Resolve.ProactiveResolver.Enabled,
				ConcurrencyLimit:         m.config.Endpoints.Resolve.ProactiveResolver.ConcurrencyLimit,
				QueueSize:                m.config.Endpoints.Resolve.ProactiveResolver.QueueSize,
				ResponseStorageDir:       m.config.Endpoints.Resolve.ProactiveResolver.ResponseStorage.Dir,
				ResponseStorageStoreJSON: m.config.Endpoints.Resolve.ProactiveResolver.ResponseStorage.StoreJSON,
				ResponseStorageStoreJWT:  m.config.Endpoints.Resolve.ProactiveResolver.ResponseStorage.StoreJWT,
			},
		}
		cfgJSON, _ := json.Marshal(cfg)
		req := model.AddFederationEndpoint{
			Type:             model.EndpointTypeResolve,
			Path:             migration.StrPtrOrNil(m.config.Endpoints.Resolve.Path),
			URL:              migration.StrPtrOrNil(m.config.Endpoints.Resolve.URL),
			AuthEnabled:      authEnabled,
			AuthTrustAnchors: authTAs,
			Config:           string(cfgJSON),
		}
		results = append(results, createOrUpdateEndpoint(req))
	}

	// Simple endpoints
	migrateSimple(model.EndpointTypeTrustMarkStatus,
		m.config.Endpoints.TrustMarkStatus.Path, m.config.Endpoints.TrustMarkStatus.URL,
		m.config.Endpoints.TrustMarkStatus.AuthEnabled || allRequireAuth,
		m.config.Endpoints.TrustMarkStatus.AuthTrustAnchors)
	migrateSimple(model.EndpointTypeTrustMarkListing,
		m.config.Endpoints.TrustMarkList.Path, m.config.Endpoints.TrustMarkList.URL,
		m.config.Endpoints.TrustMarkList.AuthEnabled || allRequireAuth,
		m.config.Endpoints.TrustMarkList.AuthTrustAnchors)
	migrateSimple(model.EndpointTypeHistoricalKeys,
		m.config.Endpoints.HistoricalKeys.Path, m.config.Endpoints.HistoricalKeys.URL,
		m.config.Endpoints.HistoricalKeys.AuthEnabled || allRequireAuth,
		m.config.Endpoints.HistoricalKeys.AuthTrustAnchors)
	migrateSimple(model.EndpointTypeTrustMark,
		m.config.Endpoints.TrustMark.Path, m.config.Endpoints.TrustMark.URL,
		m.config.Endpoints.TrustMark.AuthEnabled || allRequireAuth,
		m.config.Endpoints.TrustMark.AuthTrustAnchors)
	migrateSimple(model.EndpointTypeEnrollRequest,
		m.config.Endpoints.EnrollRequest.Path, m.config.Endpoints.EnrollRequest.URL,
		m.config.Endpoints.EnrollRequest.AuthEnabled || allRequireAuth,
		m.config.Endpoints.EnrollRequest.AuthTrustAnchors)
	migrateSimple(model.EndpointTypeTrustMarkRequest,
		m.config.Endpoints.TrustMarkRequest.Path, m.config.Endpoints.TrustMarkRequest.URL,
		m.config.Endpoints.TrustMarkRequest.AuthEnabled || allRequireAuth,
		m.config.Endpoints.TrustMarkRequest.AuthTrustAnchors)

	// Enroll (with checker config)
	if m.config.Endpoints.Enroll.Path != "" || m.config.Endpoints.Enroll.URL != "" {
		authTAs := m.config.Endpoints.Enroll.AuthTrustAnchors
		authEnabled := m.config.Endpoints.Enroll.AuthEnabled || allRequireAuth
		if authEnabled && len(authTAs) == 0 {
			authTAs = defaultTAEntityIDs
		}
		checkerType, checkerConfig := migration.ConvertCheckerConfig(&m.config.Endpoints.Enroll.Checker)
		cfgJSON := ""
		if checkerType != "" {
			cfg := migration.EnrollDBConfigJSON{
				CheckerType:   checkerType,
				CheckerConfig: checkerConfig,
			}
			b, _ := json.Marshal(cfg)
			cfgJSON = string(b)
		}
		req := model.AddFederationEndpoint{
			Type:             model.EndpointTypeEnroll,
			Path:             migration.StrPtrOrNil(m.config.Endpoints.Enroll.Path),
			URL:              migration.StrPtrOrNil(m.config.Endpoints.Enroll.URL),
			AuthEnabled:      authEnabled,
			AuthTrustAnchors: authTAs,
			Config:           cfgJSON,
		}
		results = append(results, createOrUpdateEndpoint(req))
	}

	// Entity collection
	if m.config.Endpoints.EntityCollection.Path != "" || m.config.Endpoints.EntityCollection.URL != "" {
		authTAs := m.config.Endpoints.EntityCollection.AuthTrustAnchors
		authEnabled := m.config.Endpoints.EntityCollection.AuthEnabled || allRequireAuth
		if authEnabled && len(authTAs) == 0 {
			authTAs = defaultTAEntityIDs
		}
		cfg := migration.CollectionDBConfigJSON{
			AllowedTrustAnchors: m.config.Endpoints.EntityCollection.AllowedTrustAnchors,
			IntervalSeconds:     int64(m.config.Endpoints.EntityCollection.Interval.Duration().Seconds()),
			ConcurrencyLimit:    m.config.Endpoints.EntityCollection.ConcurrencyLimit,
			PaginationLimit:     m.config.Endpoints.EntityCollection.PaginationLimit,
		}
		cfgJSON, _ := json.Marshal(cfg)
		req := model.AddFederationEndpoint{
			Type:             model.EndpointTypeEntityCollection,
			Path:             migration.StrPtrOrNil(m.config.Endpoints.EntityCollection.Path),
			URL:              migration.StrPtrOrNil(m.config.Endpoints.EntityCollection.URL),
			AuthEnabled:      authEnabled,
			AuthTrustAnchors: authTAs,
			Config:           string(cfgJSON),
		}
		results = append(results, createOrUpdateEndpoint(req))
	}

	if len(results) == 0 {
		results = append(results, migrationResult{
			section: sectionEndpoints,
			action:  "skipped",
			details: "no endpoints found in config",
		})
	}

	return results
}
