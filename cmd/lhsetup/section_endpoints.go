package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-oidfed/lighthouse/internal/migration"
	"github.com/go-oidfed/lighthouse/storage/model"
)

func sectionEndpoints() {
	printHeader("Federation Endpoints")

	endpoints, err := backends.FederationEndpoints.List()
	if err != nil {
		fmt.Printf("  Error reading: %s\n", err)
		return
	}

	existing := make(map[model.FederationEndpointType]*model.FederationEndpoint)
	for i := range endpoints {
		ep := &endpoints[i]
		existing[ep.Type] = ep
	}

	tas, err := backends.TrustAnchors.List()
	if err != nil {
		fmt.Printf("  Error listing trust anchors: %s\n", err)
		return
	}
	taEntityIDs := make([]string, 0, len(tas))
	taByEntityID := make(map[string]uint)
	for _, ta := range tas {
		taEntityIDs = append(taEntityIDs, ta.EntityID)
		taByEntityID[ta.EntityID] = ta.ID
	}

	for _, epType := range model.AllFederationEndpointTypes() {
		fmt.Println()
		configureEndpoint(epType, existing, taEntityIDs, taByEntityID)
	}
}

func configureEndpoint(
	epType model.FederationEndpointType,
	existing map[model.FederationEndpointType]*model.FederationEndpoint,
	taEntityIDs []string,
	taByEntityID map[string]uint,
) {
	printSubHeader(fmt.Sprintf("Endpoint: %s", epType))

	ep, hasExisting := existing[epType]

	path := ""
	url := ""
	authEnabled := false
	if hasExisting {
		if ep.Path != nil {
			path = *ep.Path
		}
		if ep.URL != nil {
			url = *ep.URL
		}
		authEnabled = ep.AuthEnabled
		printValue("Current path", path)
		if url != "" {
			printValue("Current URL", url)
		}
		printValue("Auth enabled", authEnabled)
	} else {
		fmt.Println("  (not configured)")
	}

	if !promptYesNo("Configure this endpoint?") {
		return
	}

	newPath := promptString("Path (Enter to keep current, '-' to disable)", path)
	if newPath == "-" {
		newPath = ""
	}
	newURL := promptString("URL (optional, Enter to keep current)", url)
	newAuthEnabled := promptBool("Auth enabled?", authEnabled)

	var authTAIDs []uint
	if newAuthEnabled {
		fmt.Println("  Available trust anchors:")
		for i, id := range taEntityIDs {
			fmt.Printf("    %d. %s\n", i+1, id)
		}
		currentTAs := []string{}
		if hasExisting && len(ep.AuthTrustAnchors) > 0 {
			for _, ta := range ep.AuthTrustAnchors {
				currentTAs = append(currentTAs, ta.EntityID)
			}
		}
		fmt.Println("  Enter trust anchor entity IDs (comma-separated, or 'all' for all):")
		taInput := promptString("Auth trust anchors", fmt.Sprintf("%s", currentTAs))
		if taInput == "all" {
			for _, id := range taEntityIDs {
				authTAIDs = append(authTAIDs, taByEntityID[id])
			}
		} else if taInput != "" {
			for _, id := range splitCommaList(taInput) {
				if dbID, ok := taByEntityID[id]; ok {
					authTAIDs = append(authTAIDs, dbID)
				} else {
					fmt.Printf("  Warning: trust anchor '%s' not found in DB, skipping.\n", id)
				}
			}
		}
	}

	var configJSON string
	switch epType {
	case model.EndpointTypeResolve:
		configJSON = configureResolveEndpoint(ep)
	case model.EndpointTypeEnroll:
		configJSON = configureEnrollEndpoint(ep)
	case model.EndpointTypeEntityCollection:
		configJSON = configureCollectionEndpoint(ep)
	}

	req := model.AddFederationEndpoint{
		Type:             epType,
		Path:             migration.StrPtrOrNil(newPath),
		URL:              migration.StrPtrOrNil(newURL),
		AuthEnabled:      newAuthEnabled,
		AuthTrustAnchors: authTAIDs,
		Config:           configJSON,
	}

	if hasExisting {
		if _, err := backends.FederationEndpoints.Update(epType, req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Updated endpoint '%s'.\n", epType)
	} else {
		if _, err := backends.FederationEndpoints.Create(req); err != nil {
			fmt.Printf("  Error: %s\n", err)
			return
		}
		fmt.Printf("  Created endpoint '%s'.\n", epType)
	}
}

func configureResolveEndpoint(ep *model.FederationEndpoint) string {
	var cfg migration.ResolveDBConfigJSON
	if ep != nil && ep.Config != "" {
		_ = json.Unmarshal([]byte(ep.Config), &cfg)
	}

	fmt.Println("  Resolve endpoint specific config:")

	cfg.AllowedTrustAnchors = promptList("Allowed trust anchors (entity IDs, comma-separated)", cfg.AllowedTrustAnchors)

	cfg.UseEntityCollectionAllowedTrustAnchors = promptBool(
		"Use entity collection allowed trust anchors?", cfg.UseEntityCollectionAllowedTrustAnchors,
	)

	currentGrace := time.Duration(cfg.GracePeriodSeconds) * time.Second
	cfg.GracePeriodSeconds = int64(promptDuration("Grace period (0=none)", currentGrace).Seconds())

	cfg.TimeElapsedGraceFactor = promptFloat("Time elapsed grace factor (0=disabled)", cfg.TimeElapsedGraceFactor)

	if promptBool("Configure proactive resolver?", false) {
		if cfg.ProactiveResolver == nil {
			cfg.ProactiveResolver = &migration.ProactiveResolverDBConfigJSON{}
		}
		cfg.ProactiveResolver.Enabled = promptBool("  Enabled?", cfg.ProactiveResolver.Enabled)
		cfg.ProactiveResolver.ConcurrencyLimit = promptInt("  Concurrency limit", cfg.ProactiveResolver.ConcurrencyLimit)
		cfg.ProactiveResolver.QueueSize = promptInt("  Queue size", cfg.ProactiveResolver.QueueSize)
		cfg.ProactiveResolver.ResponseStorageDir = promptString("  Response storage dir", cfg.ProactiveResolver.ResponseStorageDir)
		cfg.ProactiveResolver.ResponseStorageStoreJSON = promptBool("  Store JSON?", cfg.ProactiveResolver.ResponseStorageStoreJSON)
		cfg.ProactiveResolver.ResponseStorageStoreJWT = promptBool("  Store JWT?", cfg.ProactiveResolver.ResponseStorageStoreJWT)
	} else {
		cfg.ProactiveResolver = nil
	}

	b, _ := json.Marshal(cfg)
	return string(b)
}

func configureEnrollEndpoint(ep *model.FederationEndpoint) string {
	var cfg migration.EnrollDBConfigJSON
	if ep != nil && ep.Config != "" {
		_ = json.Unmarshal([]byte(ep.Config), &cfg)
	}

	fmt.Println("  Enroll endpoint specific config:")

	cfg.CheckerType = promptString("Checker type (optional, Enter to keep current)", cfg.CheckerType)

	if cfg.CheckerType != "" {
		checkerConfigPath := promptFilePath("Checker config JSON file (optional)")
		if checkerConfigPath != "" {
			data, err := os.ReadFile(checkerConfigPath)
			if err != nil {
				fmt.Printf("  Error reading checker config: %s\n", err)
			} else {
				var raw map[string]any
				if err := json.Unmarshal(data, &raw); err != nil {
					fmt.Printf("  Error parsing checker config JSON: %s\n", err)
				} else {
					cfg.CheckerConfig = raw
				}
			}
		}
	}

	if cfg.CheckerType == "" {
		return ""
	}

	b, _ := json.Marshal(cfg)
	return string(b)
}

func configureCollectionEndpoint(ep *model.FederationEndpoint) string {
	var cfg migration.CollectionDBConfigJSON
	if ep != nil && ep.Config != "" {
		_ = json.Unmarshal([]byte(ep.Config), &cfg)
	}

	fmt.Println("  Entity collection endpoint specific config:")

	cfg.AllowedTrustAnchors = promptList("Allowed trust anchors (entity IDs, comma-separated)", cfg.AllowedTrustAnchors)

	currentInterval := time.Duration(cfg.IntervalSeconds) * time.Second
	cfg.IntervalSeconds = int64(promptDuration("Collection interval", currentInterval).Seconds())

	cfg.ConcurrencyLimit = promptInt("Concurrency limit", cfg.ConcurrencyLimit)
	cfg.PaginationLimit = promptInt("Pagination limit", cfg.PaginationLimit)

	b, _ := json.Marshal(cfg)
	return string(b)
}

func splitCommaList(s string) []string {
	parts := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				parts = append(parts, trimSpaces(current))
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, trimSpaces(current))
	}
	return parts
}

func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
