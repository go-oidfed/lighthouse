package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// config2dbCmd migrates configuration values to the database
func config2dbCmd(args []string) int {
	fs := flag.NewFlagSet("config2db", flag.ExitOnError)
	var (
		configFile = fs.String("config", "", "Path to config file to migrate (required)")
		dbDriver   = fs.String("db-driver", "sqlite", "Database driver: sqlite|mysql|postgres")
		dbDSN      = fs.String("db-dsn", "", "Database DSN (for mysql/postgres)")
		dbDir      = fs.String("db-dir", "", "Data directory (for sqlite)")
		dbDebug    = fs.Bool("db-debug", false, "Enable GORM debug logging")
		force      = fs.Bool("force", false, "Overwrite existing values in DB")
		dryRun     = fs.Bool("dry-run", false, "Show what would be written without actually writing")
		only       = fs.String("only", "", "Comma-separated list of sections to migrate (default: all)")
		skip       = fs.String("skip", "", "Comma-separated list of sections to skip")
		validate   = fs.Bool("validate", true, "Validate config values before migration")
		v          = fs.Bool("v", false, "Verbose logging")
	)
	fs.StringVar(configFile, "c", "", "Path to config file to migrate (shorthand)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lhmigrate config2db --config=<config.yaml> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Migrate configuration file values to the database.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable sections:\n")
		fmt.Fprintf(os.Stderr, "  alg                - Signing algorithm (signing.alg)\n")
		fmt.Fprintf(os.Stderr, "  rsa_key_len        - RSA key length (signing.rsa_key_len)\n")
		fmt.Fprintf(os.Stderr, "  key_rotation       - Key rotation config (signing.key_rotation)\n")
		fmt.Fprintf(os.Stderr, "  constraints        - Subordinate statement constraints (federation_data.constraints)\n")
		fmt.Fprintf(os.Stderr, "  metadata_crit      - Metadata policy crit operators (federation_data.metadata_policy_crit)\n")
		fmt.Fprintf(os.Stderr, "  config_lifetime    - Entity configuration lifetime (federation_data.configuration_lifetime)\n")
		fmt.Fprintf(os.Stderr, "  authority_hints    - Authority hints (federation_data.authority_hints)\n")
		fmt.Fprintf(os.Stderr, "  metadata           - Federation entity metadata (federation_data.federation_entity_metadata)\n")
		fmt.Fprintf(os.Stderr, "  trust_mark_specs   - Trust mark specifications (endpoints.trust_mark.trust_mark_specs)\n")
		fmt.Fprintf(os.Stderr, "  trust_mark_issuers - Trust mark issuers (federation_data.trust_mark_issuers)\n")
		fmt.Fprintf(os.Stderr, "  trust_mark_owners  - Trust mark owners (federation_data.trust_mark_owners)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Migrate all config values to SQLite\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config2db --config=config.yaml --db-dir=/data/lighthouse\n\n")
		fmt.Fprintf(os.Stderr, "  # Migrate only signing options with force\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config2db -c config.yaml --db-dir=/data --only=alg,rsa_key_len,key_rotation --force\n\n")
		fmt.Fprintf(os.Stderr, "  # Dry run to see what would be migrated\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config2db -c config.yaml --db-dir=/data --dry-run -v\n")
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *v {
		log.SetLevel(log.DebugLevel)
	}

	// Validate required flags
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "--config is required")
		fs.Usage()
		return 2
	}

	// Parse sections
	sections, err := parseSections(*only)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --only: %s\n", err)
		return 2
	}

	skipSections, err := parseSkipSections(*skip)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --skip: %s\n", err)
		return 2
	}

	// Filter sections
	if len(skipSections) > 0 {
		filtered := make([]migrationSection, 0, len(sections))
		for _, s := range sections {
			if !skipSections[s] {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}

	if len(sections) == 0 {
		fmt.Fprintln(os.Stderr, "no sections to migrate")
		return 2
	}

	// Load config file
	config, err := loadMigrationConfig(*configFile)
	if err != nil {
		log.WithError(err).Error("failed to load config file")
		return 1
	}

	// Validate config if requested
	if *validate {
		if err := validateMigrationConfig(config, sections); err != nil {
			log.WithError(err).Error("config validation failed")
			return 1
		}
	}

	// Connect to database
	var driver storage.DriverType
	switch strings.ToLower(*dbDriver) {
	case string(storage.DriverSQLite):
		driver = storage.DriverSQLite
	case string(storage.DriverMySQL):
		driver = storage.DriverMySQL
	case string(storage.DriverPostgres):
		driver = storage.DriverPostgres
	default:
		fmt.Fprintf(os.Stderr, "invalid --db-driver: %s\n", *dbDriver)
		return 2
	}

	if driver == storage.DriverSQLite && *dbDir == "" {
		fmt.Fprintln(os.Stderr, "--db-dir is required for sqlite")
		return 2
	}

	if (driver == storage.DriverMySQL || driver == storage.DriverPostgres) && *dbDSN == "" {
		fmt.Fprintln(os.Stderr, "--db-dsn is required for mysql/postgres")
		return 2
	}

	if *dryRun {
		log.Info("DRY RUN - no changes will be made")
	}

	cfg := storage.Config{
		Driver:  driver,
		DSN:     *dbDSN,
		DataDir: *dbDir,
		Debug:   *dbDebug,
	}

	backs, err := storage.LoadStorageBackends(cfg)
	if err != nil {
		log.WithError(err).Error("failed to connect to database")
		return 1
	}
	log.Info("Connected to database")

	// Run migrations
	migrator := &configMigrator{
		config:   config,
		backends: backs,
		force:    *force,
		dryRun:   *dryRun,
		sections: sections,
	}

	results := migrator.migrate()

	// Print summary
	printMigrationSummary(results)

	// Check for errors
	hasErrors := false
	for _, r := range results {
		if r.err != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return 1
	}
	return 0
}

func loadMigrationConfig(filename string) (*migrationConfig, error) {
	content, err := fileutils.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	var config migrationConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse config file")
	}

	return &config, nil
}

func validateMigrationConfig(config *migrationConfig, sections []migrationSection) error {
	for _, s := range sections {
		switch s {
		case sectionAlg:
			if config.Signing.Alg != "" {
				if _, ok := jwa.LookupSignatureAlgorithm(config.Signing.Alg); !ok {
					return errors.Errorf("invalid signing algorithm: %s", config.Signing.Alg)
				}
			}
		case sectionRSAKeyLen:
			if config.Signing.RSAKeyLen != 0 {
				if config.Signing.RSAKeyLen < 2048 {
					return errors.Errorf("RSA key length must be at least 2048, got %d", config.Signing.RSAKeyLen)
				}
			}
		case sectionTrustMarkSpecs:
			for i, spec := range config.Endpoints.TrustMark.TrustMarkSpecs {
				if spec.TrustMarkType == "" {
					return errors.Errorf("trust_mark_specs[%d]: trust_mark_type is required", i)
				}
			}
		}
	}
	return nil
}

type migrationResult struct {
	section migrationSection
	action  string // "created", "skipped", "overwritten", "dry-run"
	err     error
	details string
}

type configMigrator struct {
	config   *migrationConfig
	backends model.Backends
	force    bool
	dryRun   bool
	sections []migrationSection
}

func (m *configMigrator) shouldMigrate(s migrationSection) bool {
	for _, sec := range m.sections {
		if sec == s {
			return true
		}
	}
	return false
}

func (m *configMigrator) migrate() []migrationResult {
	var results []migrationResult

	if m.shouldMigrate(sectionAlg) {
		results = append(results, m.migrateAlg())
	}
	if m.shouldMigrate(sectionRSAKeyLen) {
		results = append(results, m.migrateRSAKeyLen())
	}
	if m.shouldMigrate(sectionKeyRotation) {
		results = append(results, m.migrateKeyRotation())
	}
	if m.shouldMigrate(sectionConstraints) {
		results = append(results, m.migrateConstraints())
	}
	if m.shouldMigrate(sectionMetadataCrit) {
		results = append(results, m.migrateMetadataPolicyCrit())
	}
	if m.shouldMigrate(sectionConfigLifetime) {
		results = append(results, m.migrateConfigLifetime())
	}
	if m.shouldMigrate(sectionAuthorityHints) {
		results = append(results, m.migrateAuthorityHints()...)
	}
	if m.shouldMigrate(sectionMetadata) {
		results = append(results, m.migrateMetadata())
	}
	if m.shouldMigrate(sectionTrustMarkSpecs) {
		results = append(results, m.migrateTrustMarkSpecs()...)
	}
	if m.shouldMigrate(sectionTrustMarkIssuers) {
		results = append(results, m.migrateTrustMarkIssuers()...)
	}
	if m.shouldMigrate(sectionTrustMarkOwners) {
		results = append(results, m.migrateTrustMarkOwners()...)
	}

	return results
}

func (m *configMigrator) migrateAlg() migrationResult {
	result := migrationResult{section: sectionAlg}

	if m.config.Signing.Alg == "" {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	alg, err := storage.GetSigningAlg(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	if alg != storage.DefaultSigningAlg && !m.force {
		result.action = "skipped"
		result.details = fmt.Sprintf("already set to %s (use --force to overwrite)", alg.String())
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = fmt.Sprintf("would set to %s", m.config.Signing.Alg)
		return result
	}

	if err := storage.SetSigningAlg(m.backends.KV, storage.SigningAlgWithNbf{
		SigningAlg: m.config.Signing.Alg,
	}); err != nil {
		result.err = err
		return result
	}

	if alg != storage.DefaultSigningAlg {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = m.config.Signing.Alg
	return result
}

func (m *configMigrator) migrateRSAKeyLen() migrationResult {
	result := migrationResult{section: sectionRSAKeyLen}

	if m.config.Signing.RSAKeyLen == 0 {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetRSAKeyLen(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	// 2048 is the default
	if existing != 2048 && !m.force {
		result.action = "skipped"
		result.details = fmt.Sprintf("already set to %d (use --force to overwrite)", existing)
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = fmt.Sprintf("would set to %d", m.config.Signing.RSAKeyLen)
		return result
	}

	if err := storage.SetRSAKeyLen(m.backends.KV, m.config.Signing.RSAKeyLen); err != nil {
		result.err = err
		return result
	}

	if existing != 2048 {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = fmt.Sprintf("%d", m.config.Signing.RSAKeyLen)
	return result
}

func (m *configMigrator) migrateKeyRotation() migrationResult {
	result := migrationResult{section: sectionKeyRotation}

	// Check for legacy format first
	var rotationConfig kms.KeyRotationConfig
	hasConfig := false

	if m.config.Signing.KeyRotation.Interval.Duration() > 0 {
		rotationConfig.Enabled = m.config.Signing.KeyRotation.Enabled
		rotationConfig.Interval = m.config.Signing.KeyRotation.Interval
		rotationConfig.Overlap = m.config.Signing.KeyRotation.Overlap
		hasConfig = true
	} else if m.config.Signing.AutomaticKeyRollover.Interval.Duration() > 0 {
		// Legacy format
		rotationConfig.Enabled = m.config.Signing.AutomaticKeyRollover.Enabled
		rotationConfig.Interval = m.config.Signing.AutomaticKeyRollover.Interval
		hasConfig = true
	}

	if !hasConfig {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetKeyRotation(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	// Check if it's the default
	isDefault := !existing.Enabled && existing.Interval.Duration() == 0
	if !isDefault && !m.force {
		result.action = "skipped"
		result.details = "already set (use --force to overwrite)"
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = fmt.Sprintf("would set enabled=%v, interval=%s", rotationConfig.Enabled, rotationConfig.Interval.Duration())
		return result
	}

	if err := storage.SetKeyRotation(m.backends.KV, rotationConfig); err != nil {
		result.err = err
		return result
	}

	if !isDefault {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = fmt.Sprintf("enabled=%v, interval=%s", rotationConfig.Enabled, rotationConfig.Interval.Duration())
	return result
}

func (m *configMigrator) migrateConstraints() migrationResult {
	result := migrationResult{section: sectionConstraints}

	if m.config.Federation.Constraints == nil {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetConstraints(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	if existing != nil && !m.force {
		result.action = "skipped"
		result.details = "already set (use --force to overwrite)"
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = "would set constraints"
		return result
	}

	if err := storage.SetConstraints(m.backends.KV, m.config.Federation.Constraints); err != nil {
		result.err = err
		return result
	}

	if existing != nil {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = "constraints set"
	return result
}

func (m *configMigrator) migrateMetadataPolicyCrit() migrationResult {
	result := migrationResult{section: sectionMetadataCrit}

	if len(m.config.Federation.MetadataPolicyCrit) == 0 {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetMetadataPolicyCrit(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	if len(existing) > 0 && !m.force {
		result.action = "skipped"
		result.details = "already set (use --force to overwrite)"
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = fmt.Sprintf("would set %d operators", len(m.config.Federation.MetadataPolicyCrit))
		return result
	}

	if err := storage.SetMetadataPolicyCrit(m.backends.KV, m.config.Federation.MetadataPolicyCrit); err != nil {
		result.err = err
		return result
	}

	if len(existing) > 0 {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = fmt.Sprintf("%d operators", len(m.config.Federation.MetadataPolicyCrit))
	return result
}

func (m *configMigrator) migrateConfigLifetime() migrationResult {
	result := migrationResult{section: sectionConfigLifetime}

	if m.config.Federation.ConfigurationLifetime.Duration() == 0 {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetEntityConfigurationLifetime(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	if existing != storage.DefaultEntityConfigurationLifetime && !m.force {
		result.action = "skipped"
		result.details = fmt.Sprintf("already set to %s (use --force to overwrite)", existing)
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = fmt.Sprintf("would set to %s", m.config.Federation.ConfigurationLifetime.Duration())
		return result
	}

	if err := storage.SetEntityConfigurationLifetime(m.backends.KV, m.config.Federation.ConfigurationLifetime.Duration()); err != nil {
		result.err = err
		return result
	}

	if existing != storage.DefaultEntityConfigurationLifetime {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = m.config.Federation.ConfigurationLifetime.Duration().String()
	return result
}

func (m *configMigrator) migrateAuthorityHints() []migrationResult {
	var results []migrationResult

	if len(m.config.Federation.AuthorityHints) == 0 {
		results = append(results, migrationResult{
			section: sectionAuthorityHints,
			action:  "skipped",
			details: "not set in config",
		})
		return results
	}

	// Get existing hints
	existingHints, err := storage.GetAuthorityHints(m.backends.AuthorityHints)
	if err != nil {
		results = append(results, migrationResult{
			section: sectionAuthorityHints,
			err:     err,
		})
		return results
	}

	existingMap := make(map[string]bool)
	for _, h := range existingHints {
		existingMap[h] = true
	}

	for _, hint := range m.config.Federation.AuthorityHints {
		result := migrationResult{
			section: sectionAuthorityHints,
			details: hint,
		}

		if existingMap[hint] && !m.force {
			result.action = "skipped"
			result.details = fmt.Sprintf("%s already exists", hint)
			results = append(results, result)
			continue
		}

		if m.dryRun {
			result.action = "dry-run"
			result.details = fmt.Sprintf("would add %s", hint)
			results = append(results, result)
			continue
		}

		_, err := m.backends.AuthorityHints.Create(model.AddAuthorityHint{
			EntityID:    hint,
			Description: "Migrated from config file",
		})
		if err != nil {
			// Check if it's a duplicate error
			if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "duplicate") {
				if m.force {
					// Try to update instead
					if _, err := m.backends.AuthorityHints.Update(hint, model.AddAuthorityHint{
						EntityID:    hint,
						Description: "Migrated from config file (updated)",
					}); err != nil {
						result.err = err
					} else {
						result.action = "overwritten"
					}
				} else {
					result.action = "skipped"
					result.details = fmt.Sprintf("%s already exists", hint)
				}
			} else {
				result.err = err
			}
		} else {
			result.action = "created"
		}

		results = append(results, result)
	}

	return results
}

func (m *configMigrator) migrateMetadata() migrationResult {
	result := migrationResult{section: sectionMetadata}

	metadata := m.config.Federation.Metadata.ToOIDFedMetadata()
	if metadata == nil {
		result.action = "skipped"
		result.details = "not set in config"
		return result
	}

	// Check if exists
	existing, err := storage.GetMetadata(m.backends.KV)
	if err != nil {
		result.err = err
		return result
	}

	if existing != nil && existing.FederationEntity != nil && !m.force {
		result.action = "skipped"
		result.details = "already set (use --force to overwrite)"
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		result.details = "would set federation entity metadata"
		return result
	}

	if err := storage.SetMetadata(m.backends.KV, metadata); err != nil {
		result.err = err
		return result
	}

	if existing != nil && existing.FederationEntity != nil {
		result.action = "overwritten"
	} else {
		result.action = "created"
	}
	result.details = "federation entity metadata set"
	return result
}

func (m *configMigrator) migrateTrustMarkSpecs() []migrationResult {
	var results []migrationResult

	if len(m.config.Endpoints.TrustMark.TrustMarkSpecs) == 0 {
		results = append(results, migrationResult{
			section: sectionTrustMarkSpecs,
			action:  "skipped",
			details: "not set in config",
		})
		return results
	}

	// Get existing specs
	existingSpecs, err := m.backends.TrustMarkSpecs.List()
	if err != nil {
		results = append(results, migrationResult{
			section: sectionTrustMarkSpecs,
			err:     err,
		})
		return results
	}

	existingMap := make(map[string]bool)
	for _, s := range existingSpecs {
		existingMap[s.TrustMarkType] = true
	}

	for _, spec := range m.config.Endpoints.TrustMark.TrustMarkSpecs {
		result := migrationResult{
			section: sectionTrustMarkSpecs,
			details: spec.TrustMarkType,
		}

		if existingMap[spec.TrustMarkType] && !m.force {
			result.action = "skipped"
			result.details = fmt.Sprintf("%s already exists", spec.TrustMarkType)
			results = append(results, result)
			continue
		}

		if m.dryRun {
			result.action = "dry-run"
			result.details = fmt.Sprintf("would add %s", spec.TrustMarkType)
			results = append(results, result)
			continue
		}

		newSpec := &model.TrustMarkSpec{
			TrustMarkType: spec.TrustMarkType,
			Lifetime:      spec.Lifetime,
			Ref:           spec.Ref,
			LogoURI:       spec.LogoURI,
			DelegationJWT: spec.DelegationJWT,
			Description:   "Migrated from config file",
		}

		// Convert checker config to eligibility config if present
		if spec.Checker != nil {
			newSpec.EligibilityConfig = &model.EligibilityConfig{
				Mode: model.EligibilityModeCustom,
				Checker: &model.CheckerConfig{
					Type:   spec.Checker.Type,
					Config: spec.Checker.Config,
				},
			}
		}

		if existingMap[spec.TrustMarkType] {
			// Update existing
			if _, err := m.backends.TrustMarkSpecs.Update(spec.TrustMarkType, newSpec); err != nil {
				result.err = err
			} else {
				result.action = "overwritten"
			}
		} else {
			// Create new
			if _, err := m.backends.TrustMarkSpecs.Create(newSpec); err != nil {
				result.err = err
			} else {
				result.action = "created"
			}
		}

		results = append(results, result)
	}

	return results
}

func (m *configMigrator) migrateTrustMarkIssuers() []migrationResult {
	var results []migrationResult

	if len(m.config.Federation.TrustMarkIssuers) == 0 {
		results = append(results, migrationResult{
			section: sectionTrustMarkIssuers,
			action:  "skipped",
			details: "not set in config",
		})
		return results
	}

	// For each trust mark type -> list of issuers
	for trustMarkType, issuers := range m.config.Federation.TrustMarkIssuers {
		// First ensure the trust mark type exists
		existingType, err := m.backends.TrustMarkTypes.Get(trustMarkType)
		if err != nil || existingType == nil {
			// Create the trust mark type first
			if m.dryRun {
				results = append(results, migrationResult{
					section: sectionTrustMarkIssuers,
					action:  "dry-run",
					details: fmt.Sprintf("would create type %s with %d issuers", trustMarkType, len(issuers)),
				})
				continue
			}

			// Create the type
			_, err = m.backends.TrustMarkTypes.Create(model.AddTrustMarkType{
				TrustMarkType: trustMarkType,
				Description:   "Migrated from config file",
			})
			if err != nil {
				results = append(results, migrationResult{
					section: sectionTrustMarkIssuers,
					err:     fmt.Errorf("failed to create trust mark type %s: %w", trustMarkType, err),
				})
				continue
			}
		}

		// Now add the issuers
		for _, issuerEntityID := range issuers {
			result := migrationResult{
				section: sectionTrustMarkIssuers,
				details: fmt.Sprintf("%s: %s", trustMarkType, issuerEntityID),
			}

			// Check if issuer already exists for this type
			existingIssuers, err := m.backends.TrustMarkTypes.ListIssuers(trustMarkType)
			if err != nil {
				result.err = fmt.Errorf("failed to list issuers: %w", err)
				results = append(results, result)
				continue
			}

			issuerExists := false
			for _, existing := range existingIssuers {
				if existing.Issuer == issuerEntityID {
					issuerExists = true
					break
				}
			}

			if issuerExists && !m.force {
				result.action = "skipped"
				result.details = fmt.Sprintf("%s: %s already exists", trustMarkType, issuerEntityID)
				results = append(results, result)
				continue
			}

			if m.dryRun {
				result.action = "dry-run"
				result.details = fmt.Sprintf("would add issuer %s to type %s", issuerEntityID, trustMarkType)
				results = append(results, result)
				continue
			}

			// Add the issuer
			_, err = m.backends.TrustMarkTypes.AddIssuer(trustMarkType, model.AddTrustMarkIssuer{
				Issuer:      issuerEntityID,
				Description: "Migrated from config file",
			})
			if err != nil {
				result.err = fmt.Errorf("failed to add issuer: %w", err)
			} else {
				result.action = "created"
			}
			results = append(results, result)
		}
	}

	return results
}

func (m *configMigrator) migrateTrustMarkOwners() []migrationResult {
	var results []migrationResult

	if len(m.config.Federation.TrustMarkOwners) == 0 {
		results = append(results, migrationResult{
			section: sectionTrustMarkOwners,
			action:  "skipped",
			details: "not set in config",
		})
		return results
	}

	// For each trust mark type -> owner spec
	for trustMarkType, ownerConfig := range m.config.Federation.TrustMarkOwners {
		result := migrationResult{
			section: sectionTrustMarkOwners,
			details: fmt.Sprintf("%s: %s", trustMarkType, ownerConfig.EntityID),
		}

		// First ensure the trust mark type exists
		existingType, err := m.backends.TrustMarkTypes.Get(trustMarkType)
		if err != nil || existingType == nil {
			// Create the trust mark type first
			if m.dryRun {
				result.action = "dry-run"
				result.details = fmt.Sprintf("would create type %s with owner %s", trustMarkType, ownerConfig.EntityID)
				results = append(results, result)
				continue
			}

			// Create the type
			_, err = m.backends.TrustMarkTypes.Create(model.AddTrustMarkType{
				TrustMarkType: trustMarkType,
				Description:   "Migrated from config file",
			})
			if err != nil {
				result.err = fmt.Errorf("failed to create trust mark type %s: %w", trustMarkType, err)
				results = append(results, result)
				continue
			}
		}

		// Check if owner already exists for this type
		existingOwner, err := m.backends.TrustMarkTypes.GetOwner(trustMarkType)
		if err == nil && existingOwner != nil && !m.force {
			result.action = "skipped"
			result.details = fmt.Sprintf("%s: owner already set to %s", trustMarkType, existingOwner.EntityID)
			results = append(results, result)
			continue
		}

		if m.dryRun {
			result.action = "dry-run"
			result.details = fmt.Sprintf("would set owner %s for type %s", ownerConfig.EntityID, trustMarkType)
			results = append(results, result)
			continue
		}

		// Create or update the owner
		ownerReq := model.AddTrustMarkOwner{
			EntityID: ownerConfig.EntityID,
		}

		// Try to parse JWKS if provided
		if ownerConfig.JWKS != nil {
			// The JWKS might be in various formats; we'll try to handle it
			jwksData, err := json.Marshal(ownerConfig.JWKS)
			if err == nil {
				var jwks model.JWKS
				if err := json.Unmarshal(jwksData, &jwks); err == nil {
					ownerReq.JWKS = jwks
				}
			}
		}

		if existingOwner != nil {
			// Update existing
			_, err = m.backends.TrustMarkTypes.UpdateOwner(trustMarkType, ownerReq)
			if err != nil {
				result.err = fmt.Errorf("failed to update owner: %w", err)
			} else {
				result.action = "overwritten"
			}
		} else {
			// Create new
			_, err = m.backends.TrustMarkTypes.CreateOwner(trustMarkType, ownerReq)
			if err != nil {
				result.err = fmt.Errorf("failed to create owner: %w", err)
			} else {
				result.action = "created"
			}
		}
		results = append(results, result)
	}

	return results
}

func printMigrationSummary(results []migrationResult) {
	fmt.Println("\n=== Migration Summary ===")
	fmt.Println()

	created := 0
	skipped := 0
	overwritten := 0
	dryRuns := 0
	errors := 0

	for _, r := range results {
		var status string
		switch r.action {
		case "created":
			status = "[CREATED]"
			created++
		case "skipped":
			status = "[SKIPPED]"
			skipped++
		case "overwritten":
			status = "[UPDATED]"
			overwritten++
		case "dry-run":
			status = "[DRY-RUN]"
			dryRuns++
		default:
			if r.err != nil {
				status = "[ERROR]"
				errors++
			} else {
				status = "[UNKNOWN]"
			}
		}

		if r.err != nil {
			fmt.Printf("  %-20s %s - ERROR: %s\n", r.section, status, r.err)
		} else {
			fmt.Printf("  %-20s %s %s\n", r.section, status, r.details)
		}
	}

	fmt.Println()
	fmt.Printf("Total: %d created, %d updated, %d skipped, %d errors\n", created, overwritten, skipped, errors)
	if dryRuns > 0 {
		fmt.Printf("       %d would be changed (dry-run)\n", dryRuns)
	}
}
