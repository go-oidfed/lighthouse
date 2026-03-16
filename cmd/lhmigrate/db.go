package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// dbMigrationSection represents which sections of data to migrate
type dbMigrationSection string

const (
	dbSectionSubordinates        dbMigrationSection = "subordinates"
	dbSectionTrustMarkedEntities dbMigrationSection = "trust_marked_entities"
)

// allDBSections returns all available db migration sections
func allDBSections() []dbMigrationSection {
	return []dbMigrationSection{
		dbSectionSubordinates,
		dbSectionTrustMarkedEntities,
	}
}

// parseDBSections parses a comma-separated list of sections
func parseDBSections(s string) ([]dbMigrationSection, error) {
	if s == "" || s == "all" {
		return allDBSections(), nil
	}

	parts := splitAndTrim(s, ",")
	sections := make([]dbMigrationSection, 0, len(parts))

	for _, p := range parts {
		sec := dbMigrationSection(p)
		if !isValidDBSection(sec) {
			return nil, fmt.Errorf("invalid section: %s", p)
		}
		sections = append(sections, sec)
	}
	return sections, nil
}

func isValidDBSection(s dbMigrationSection) bool {
	for _, valid := range allDBSections() {
		if s == valid {
			return true
		}
	}
	return false
}

// dbMigrationResult holds the result of a single migration operation
type dbMigrationResult struct {
	section  dbMigrationSection
	entityID string
	action   string // "created", "skipped", "updated", "error", "dry-run"
	err      error
	details  string
}

// dbMigrator handles the migration of legacy storage to GORM database
type dbMigrator struct {
	sourceType string // "json" or "badger"
	sourceDir  string
	backends   model.Backends
	force      bool
	dryRun     bool
	verbose    bool
	sections   []dbMigrationSection
	results    []dbMigrationResult
}

// shouldMigrate checks if a section should be migrated
func (m *dbMigrator) shouldMigrate(s dbMigrationSection) bool {
	for _, sec := range m.sections {
		if sec == s {
			return true
		}
	}
	return false
}

// migrate runs the migration for all enabled sections
func (m *dbMigrator) migrate() error {
	if m.shouldMigrate(dbSectionSubordinates) {
		if err := m.migrateSubordinates(); err != nil {
			return fmt.Errorf("subordinates migration failed: %w", err)
		}
	}

	if m.shouldMigrate(dbSectionTrustMarkedEntities) {
		if err := m.migrateTrustMarkedEntities(); err != nil {
			return fmt.Errorf("trust marked entities migration failed: %w", err)
		}
	}

	return nil
}

// loadLegacySubordinates loads subordinates from the appropriate legacy storage
func (m *dbMigrator) loadLegacySubordinates() ([]legacySubordinateInfo, error) {
	switch m.sourceType {
	case "json":
		store := NewFileStorage(m.sourceDir)
		loader := store.subordinateStorage()
		return loader()
	case "badger":
		store, err := NewBadgerStorage(m.sourceDir)
		if err != nil {
			return nil, fmt.Errorf("failed to open badger storage: %w", err)
		}
		loader := store.subordinateStorage()
		return loader()
	default:
		return nil, fmt.Errorf("unsupported source type: %s", m.sourceType)
	}
}

// migrateSubordinates migrates subordinate data from legacy storage
func (m *dbMigrator) migrateSubordinates() error {
	log.Info("Migrating subordinates...")

	legacyInfos, err := m.loadLegacySubordinates()
	if err != nil {
		// Check if file doesn't exist - that's OK, just skip
		if os.IsNotExist(err) {
			log.Info("No subordinates file found, skipping")
			m.results = append(m.results, dbMigrationResult{
				section: dbSectionSubordinates,
				action:  "skipped",
				details: "no subordinates file found",
			})
			return nil
		}
		return err
	}

	log.WithField("count", len(legacyInfos)).Info("Found legacy subordinates")

	for _, legacy := range legacyInfos {
		result := m.migrateOneSubordinate(legacy)
		m.results = append(m.results, result)

		if m.verbose && result.err == nil {
			log.WithFields(log.Fields{
				"entity_id": result.entityID,
				"action":    result.action,
			}).Debug("Processed subordinate")
		}
	}

	return nil
}

// migrateOneSubordinate migrates a single subordinate
func (m *dbMigrator) migrateOneSubordinate(legacy legacySubordinateInfo) dbMigrationResult {
	result := dbMigrationResult{
		section:  dbSectionSubordinates,
		entityID: legacy.EntityID,
	}

	// Check if subordinate already exists
	existing, err := m.backends.Subordinates.Get(legacy.EntityID)
	if err != nil && !isNotFoundError(err) {
		result.err = fmt.Errorf("failed to check existing subordinate: %w", err)
		return result
	}

	if existing != nil && !m.force {
		result.action = "skipped"
		result.details = "already exists (use --force to overwrite)"
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		if existing != nil {
			result.details = "would overwrite existing"
		} else {
			result.details = "would create"
		}
		return result
	}

	// Transform legacy to new format
	newInfo := transformSubordinate(legacy)

	// Warn about MetadataPolicyCrit if set (no longer per-subordinate)
	if len(legacy.MetadataPolicyCrit) > 0 {
		log.WithField("entity_id", legacy.EntityID).Warn(
			"MetadataPolicyCrit is no longer per-subordinate; consider migrating to global setting via config2db",
		)
	}

	if existing != nil {
		// Update existing
		if err = m.backends.Subordinates.Update(legacy.EntityID, newInfo); err != nil {
			result.err = fmt.Errorf("failed to update subordinate: %w", err)
			return result
		}
		result.action = "updated"
	} else {
		// Create new
		if err = m.backends.Subordinates.Add(newInfo); err != nil {
			result.err = fmt.Errorf("failed to create subordinate: %w", err)
			return result
		}
		result.action = "created"
	}

	return result
}

// transformSubordinate converts a legacy subordinate to the new format
func transformSubordinate(legacy legacySubordinateInfo) model.ExtendedSubordinateInfo {
	// Convert entity types to join table format
	entityTypes := make([]model.SubordinateEntityType, len(legacy.EntityTypes))
	for i, et := range legacy.EntityTypes {
		entityTypes[i] = model.SubordinateEntityType{
			EntityType: et,
		}
	}

	return model.ExtendedSubordinateInfo{
		BasicSubordinateInfo: model.BasicSubordinateInfo{
			EntityID:               legacy.EntityID,
			Description:            "Migrated from legacy storage",
			SubordinateEntityTypes: entityTypes,
			Status:                 legacy.Status,
		},
		JWKS:           model.NewJWKS(legacy.JWKS),
		Metadata:       legacy.Metadata,
		MetadataPolicy: legacy.MetadataPolicy,
		Constraints:    legacy.Constraints,
	}
}

// migrateTrustMarkedEntities migrates trust marked entities from legacy storage
func (m *dbMigrator) migrateTrustMarkedEntities() error {
	log.Info("Migrating trust marked entities...")

	var tmeStorage model.TrustMarkedEntitiesStorageBackend
	switch m.sourceType {
	case "json":
		store := NewFileStorage(m.sourceDir)
		tmeStorage = store.TrustMarkedEntitiesStorage()
	case "badger":
		store, err := NewBadgerStorage(m.sourceDir)
		if err != nil {
			return fmt.Errorf("failed to open badger storage: %w", err)
		}
		tmeStorage = store.TrustMarkedEntitiesStorage()
	default:
		return fmt.Errorf("unsupported source type: %s", m.sourceType)
	}

	if err := tmeStorage.Load(); err != nil {
		if os.IsNotExist(err) {
			log.Info("No trust marked entities file found, skipping")
			m.results = append(m.results, dbMigrationResult{
				section: dbSectionTrustMarkedEntities,
				action:  "skipped",
				details: "no trust marked entities file found",
			})
			return nil
		}
		return fmt.Errorf("failed to load legacy trust marked entities: %w", err)
	}

	// Get all trust mark specs to know which types exist
	specs, err := m.backends.TrustMarkSpecs.List()
	if err != nil {
		return fmt.Errorf("failed to list trust mark specs: %w", err)
	}

	if len(specs) == 0 {
		log.Warn("No trust mark specs found in database. Trust marked entities require specs to be migrated first (use config2db).")
		m.results = append(m.results, dbMigrationResult{
			section: dbSectionTrustMarkedEntities,
			action:  "skipped",
			details: "no trust mark specs found - run config2db first",
		})
		return nil
	}

	// Process each trust mark spec
	for _, spec := range specs {
		// Get active entities for this trust mark type
		activeEntities, err := tmeStorage.Active(spec.TrustMarkType)
		if err != nil {
			log.WithError(err).WithField("trust_mark_type", spec.TrustMarkType).Warn("Failed to get active entities")
			continue
		}
		for _, entityID := range activeEntities {
			result := m.migrateOneTrustMarkedEntity(spec.TrustMarkType, entityID, model.StatusActive)
			m.results = append(m.results, result)
		}

		// Get blocked entities
		blockedEntities, err := tmeStorage.Blocked(spec.TrustMarkType)
		if err != nil {
			log.WithError(err).WithField("trust_mark_type", spec.TrustMarkType).Warn("Failed to get blocked entities")
			continue
		}
		for _, entityID := range blockedEntities {
			result := m.migrateOneTrustMarkedEntity(spec.TrustMarkType, entityID, model.StatusBlocked)
			m.results = append(m.results, result)
		}

		// Get pending entities
		pendingEntities, err := tmeStorage.Pending(spec.TrustMarkType)
		if err != nil {
			log.WithError(err).WithField("trust_mark_type", spec.TrustMarkType).Warn("Failed to get pending entities")
			continue
		}
		for _, entityID := range pendingEntities {
			result := m.migrateOneTrustMarkedEntity(spec.TrustMarkType, entityID, model.StatusPending)
			m.results = append(m.results, result)
		}
	}

	return nil
}

// migrateOneTrustMarkedEntity migrates a single trust marked entity
func (m *dbMigrator) migrateOneTrustMarkedEntity(trustMarkType, entityID string, status model.Status) dbMigrationResult {
	result := dbMigrationResult{
		section:  dbSectionTrustMarkedEntities,
		entityID: fmt.Sprintf("%s:%s", trustMarkType, entityID),
	}

	// Check if subject already exists
	existing, err := m.backends.TrustMarkSpecs.GetSubject(trustMarkType, entityID)
	if err != nil && !isNotFoundError(err) {
		result.err = fmt.Errorf("failed to check existing subject: %w", err)
		return result
	}

	if existing != nil && !m.force {
		result.action = "skipped"
		result.details = fmt.Sprintf("already exists with status %s (use --force to overwrite)", existing.Status.String())
		return result
	}

	if m.dryRun {
		result.action = "dry-run"
		if existing != nil {
			result.details = fmt.Sprintf("would update status to %s", status.String())
		} else {
			result.details = fmt.Sprintf("would create with status %s", status.String())
		}
		return result
	}

	if existing != nil {
		// Update existing - change status
		if _, err := m.backends.TrustMarkSpecs.ChangeSubjectStatus(trustMarkType, entityID, status); err != nil {
			result.err = fmt.Errorf("failed to update subject status: %w", err)
			return result
		}
		result.action = "updated"
		result.details = fmt.Sprintf("status -> %s", status.String())
	} else {
		// Create new subject
		subject := &model.TrustMarkSubject{
			EntityID:    entityID,
			Status:      status,
			Description: "Migrated from legacy storage",
		}
		if _, err := m.backends.TrustMarkSpecs.CreateSubject(trustMarkType, subject); err != nil {
			result.err = fmt.Errorf("failed to create subject: %w", err)
			return result
		}
		result.action = "created"
		result.details = fmt.Sprintf("status: %s", status.String())
	}

	return result
}

// printDBMigrationSummary prints a summary of the migration results
func printDBMigrationSummary(results []dbMigrationResult) {
	fmt.Println("\n=== Database Migration Summary ===")
	fmt.Println()

	created := 0
	skipped := 0
	updated := 0
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
		case "updated":
			status = "[UPDATED]"
			updated++
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

		entityDisplay := r.entityID
		if entityDisplay == "" {
			entityDisplay = string(r.section)
		}

		if r.err != nil {
			fmt.Printf("  %-40s %s - ERROR: %s\n", entityDisplay, status, r.err)
		} else if r.details != "" {
			fmt.Printf("  %-40s %s %s\n", entityDisplay, status, r.details)
		} else {
			fmt.Printf("  %-40s %s\n", entityDisplay, status)
		}
	}

	fmt.Println()
	fmt.Printf("Total: %d created, %d updated, %d skipped, %d errors\n", created, updated, skipped, errors)
	if dryRuns > 0 {
		fmt.Printf("       %d would be changed (dry-run)\n", dryRuns)
	}
}

// isNotFoundError checks if an error indicates a record was not found
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "record not found") ||
		strings.Contains(errStr, "no rows")
}

// runDBMigration is the main entry point for the db migration command
func runDBMigration(args []string) int {
	fs := flag.NewFlagSet("db", flag.ExitOnError)
	var (
		srcType string
		srcDir  string
		dbType  string
		destDir string
		dbDSN   string
		dbDebug bool
		force   bool
		dryRun  bool
		only    string
		skip    string
		verbose bool
	)
	// --source-type
	fs.StringVar(&srcType, "source-type", "", "Source storage type: json|badger")
	// --source / -s
	fs.StringVar(&srcDir, "source", "", "Source data directory")
	fs.StringVar(&srcDir, "s", "", "Source data directory (shorthand)")
	// --db-type
	fs.StringVar(&dbType, "db-type", "sqlite", "Destination database type: sqlite|mysql|postgres")
	// --dest / -d
	fs.StringVar(&destDir, "dest", "", "Destination data directory (for sqlite)")
	fs.StringVar(&destDir, "d", "", "Destination data directory (shorthand)")
	// --db-dsn
	fs.StringVar(&dbDSN, "db-dsn", "", "Destination DSN (for mysql/postgres)")
	// --db-debug
	fs.BoolVar(&dbDebug, "db-debug", false, "Enable GORM debug logging")
	// --force / -f
	fs.BoolVar(&force, "force", false, "Overwrite existing records")
	fs.BoolVar(&force, "f", false, "Overwrite existing records (shorthand)")
	// --dry-run / -n
	fs.BoolVar(&dryRun, "dry-run", false, "Preview only, don't make changes")
	fs.BoolVar(&dryRun, "n", false, "Preview only (shorthand)")
	// --only
	fs.StringVar(&only, "only", "", "Comma-separated list of sections to migrate (default: all)")
	// --skip
	fs.StringVar(&skip, "skip", "", "Comma-separated list of sections to skip")
	// --verbose / -v
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lhmigrate db --source-type=<json|badger> --source=<dir> --db-type=<sqlite|mysql|postgres> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Migrate legacy storage data to GORM-based database.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable sections:\n")
		fmt.Fprintf(os.Stderr, "  subordinates          - Subordinate entities and their JWKS\n")
		fmt.Fprintf(os.Stderr, "  trust_marked_entities - Trust mark subject eligibility status\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Migrate JSON file storage to SQLite\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate db --source-type=json --source=/old/data --db-type=sqlite --dest=/new/data\n\n")
		fmt.Fprintf(os.Stderr, "  # Migrate BadgerDB to PostgreSQL\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate db --source-type=badger -s /old/badger --db-type=postgres \\\n")
		fmt.Fprintf(os.Stderr, "    --db-dsn='host=localhost user=lh password=secret dbname=lighthouse'\n\n")
		fmt.Fprintf(os.Stderr, "  # Dry run to preview migration\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate db --source-type=json -s /old/data -d /new/data -n -v\n")
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	// Validate required flags
	if srcType == "" {
		fmt.Fprintln(os.Stderr, "--source-type is required")
		fs.Usage()
		return 2
	}
	if srcType != "json" && srcType != "badger" {
		fmt.Fprintf(os.Stderr, "invalid --source-type: %s (must be json or badger)\n", srcType)
		return 2
	}
	if srcDir == "" {
		fmt.Fprintln(os.Stderr, "--source is required")
		fs.Usage()
		return 2
	}

	// Parse destination database type
	driver, err := storage.ParseDriverType(strings.ToLower(dbType))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --db-type: %s\n", err)
		return 2
	}

	if err = validateDBFlags(driver, destDir, dbDSN); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	// Parse sections
	sections, err := parseDBSections(only)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --only: %s\n", err)
		return 2
	}

	// Parse skip sections
	if skip != "" {
		skipSections, err := parseDBSections(skip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --skip: %s\n", err)
			return 2
		}
		skipMap := make(map[dbMigrationSection]bool)
		for _, s := range skipSections {
			skipMap[s] = true
		}
		filtered := make([]dbMigrationSection, 0, len(sections))
		for _, s := range sections {
			if !skipMap[s] {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}

	if len(sections) == 0 {
		fmt.Fprintln(os.Stderr, "no sections to migrate")
		return 2
	}

	if dryRun {
		log.Info("DRY RUN - no changes will be made")
	}

	// Connect to destination database
	cfg := storage.Config{
		Driver:  driver,
		DSN:     dbDSN,
		DataDir: destDir,
		Debug:   dbDebug,
	}

	backends, err := storage.LoadStorageBackends(cfg)
	if err != nil {
		log.WithError(err).Error("failed to connect to destination database")
		return 1
	}
	log.Info("Connected to destination database")

	// Create migrator and run
	migrator := &dbMigrator{
		sourceType: srcType,
		sourceDir:  srcDir,
		backends:   backends,
		force:      force,
		dryRun:     dryRun,
		verbose:    verbose,
		sections:   sections,
	}

	if err := migrator.migrate(); err != nil {
		log.WithError(err).Error("Migration failed")
		return 1
	}

	// Print summary
	printDBMigrationSummary(migrator.results)

	// Check for errors
	hasErrors := false
	for _, r := range migrator.results {
		if r.err != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return 1
	}

	log.Info("Database migration completed successfully")
	return 0
}
