package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// allStep represents a migration step in the all command
type allStep string

const (
	stepConfig        allStep = "config"
	stepKeysPublic    allStep = "keys-public"
	stepKeysKMS       allStep = "keys-kms"
	stepConfig2DB     allStep = "config2db"
	stepDB            allStep = "db"
	stepConfigCleanup allStep = "config-cleanup"
)

// allSteps returns all steps in execution order
func allSteps() []allStep {
	return []allStep{
		stepConfig,
		stepKeysPublic,
		stepKeysKMS,
		stepConfig2DB,
		stepDB,
		stepConfigCleanup,
	}
}

// stepName returns a human-readable name for the step
func stepName(s allStep) string {
	switch s {
	case stepConfig:
		return "Config File Transformation"
	case stepKeysPublic:
		return "Public Keys Migration"
	case stepKeysKMS:
		return "KMS Migration"
	case stepConfig2DB:
		return "Config Values to Database Migration"
	case stepDB:
		return "Storage Data Migration"
	case stepConfigCleanup:
		return "Config File Cleanup"
	default:
		return string(s)
	}
}

// parseSkipSteps parses a comma-separated list of steps to skip
func parseSkipSteps(s string) (map[allStep]bool, error) {
	if s == "" {
		return nil, nil
	}

	parts := splitAndTrim(s, ",")
	skip := make(map[allStep]bool, len(parts))

	validSteps := allSteps()
	for _, p := range parts {
		step := allStep(p)
		valid := false
		for _, vs := range validSteps {
			if step == vs {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid step: %s (valid steps: %s)", p, strings.Join(stepNames(), ", "))
		}
		skip[step] = true
	}
	return skip, nil
}

// stepNames returns all step names as strings
func stepNames() []string {
	steps := allSteps()
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = string(s)
	}
	return names
}

// allCmd is the entry point for the "all" subcommand
func allCmd(args []string) int {
	fs := flag.NewFlagSet("all", flag.ExitOnError)
	var (
		// Config file options
		configFile string
		destConfig string

		// Keys source/destination
		keysSource string
		keysDest   string

		// Data source/destination
		dataSource string
		dataDest   string
		sourceType string

		// Database options
		dbType  string
		dbDSN   string
		dbDir   string
		dbDebug bool

		// Keys options
		keyType   string
		algsStr   string
		pksType   string
		genKeys   bool
		rsaKeyLen int
		defAlg    string

		// Control options
		skipSteps string
		force     bool
		dryRun    bool
		verbose   bool
	)

	// Config file options
	fs.StringVar(&configFile, "config", "", "Path to config file (required for config, config2db, and cleanup steps)")
	fs.StringVar(&configFile, "c", "", "Path to config file (shorthand)")
	fs.StringVar(&destConfig, "dest-config", "", "Path to write transformed config (default: overwrite source)")

	// Keys source/destination
	fs.StringVar(&keysSource, "keys-source", "", "Source directory for legacy keys (PEM files, JWKS)")
	fs.StringVar(&keysDest, "keys-dest", "", "Destination directory for migrated keys (filesystem KMS)")

	// Data source/destination
	fs.StringVar(&dataSource, "data-source", "", "Source directory for legacy data (subordinates, trust marks)")
	fs.StringVar(&dataDest, "data-dest", "", "Destination directory for SQLite database (defaults to --db-dir)")
	fs.StringVar(&sourceType, "source-type", "", "Legacy storage type: json|badger (for db step)")

	// Database options
	fs.StringVar(&dbType, "db-type", "sqlite", "Database type: sqlite|mysql|postgres")
	fs.StringVar(&dbDSN, "db-dsn", "", "Database DSN (for mysql/postgres)")
	fs.StringVar(&dbDir, "db-dir", "", "Data directory for sqlite database")
	fs.BoolVar(&dbDebug, "db-debug", false, "Enable GORM debug logging")

	// Keys options
	fs.StringVar(&keyType, "key-type", "federation", "Key type identifier (e.g., 'federation')")
	fs.StringVar(&keyType, "t", "federation", "Key type identifier (shorthand)")
	fs.StringVar(&algsStr, "algs", "", "Comma-separated list of algorithms to migrate (e.g., ES256,RS256)")
	fs.StringVar(&algsStr, "a", "", "Algorithms (shorthand)")
	fs.StringVar(&pksType, "pks-type", "", "Public key storage type: fs|db (for keys kms step)")
	fs.BoolVar(&genKeys, "generate-missing", false, "Generate missing keys in KMS migration")
	fs.BoolVar(&genKeys, "g", false, "Generate missing keys (shorthand)")
	fs.IntVar(&rsaKeyLen, "rsa-len", 4096, "RSA key length when generating keys")
	fs.StringVar(&defAlg, "default-alg", "", "Default algorithm for KMS")

	// Control options
	fs.StringVar(&skipSteps, "skip", "", "Comma-separated list of steps to skip (config,keys-public,keys-kms,config2db,db,config-cleanup)")
	fs.BoolVar(&force, "force", false, "Force overwrite existing values")
	fs.BoolVar(&force, "f", false, "Force overwrite (shorthand)")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview only, don't make changes")
	fs.BoolVar(&dryRun, "n", false, "Preview only (shorthand)")
	fs.BoolVar(&verbose, "verbose", false, "Verbose logging")
	fs.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lhmigrate all [options]\n\n")
		fmt.Fprintf(os.Stderr, "Run all migration steps in sequence.\n\n")
		fmt.Fprintf(os.Stderr, "This command combines all migration steps into a single operation:\n")
		fmt.Fprintf(os.Stderr, "  1. config        - Transform legacy config file to new format\n")
		fmt.Fprintf(os.Stderr, "  2. keys-public   - Migrate public keys (JWKS + history)\n")
		fmt.Fprintf(os.Stderr, "  3. keys-kms      - Migrate private keys to filesystem KMS\n")
		fmt.Fprintf(os.Stderr, "  4. config2db     - Migrate config values to database\n")
		fmt.Fprintf(os.Stderr, "  5. db            - Migrate legacy storage data (subordinates, trust marks)\n")
		fmt.Fprintf(os.Stderr, "  6. config-cleanup - Remove empty/leftover values from config file\n")
		fmt.Fprintf(os.Stderr, "\nThe migration stops on the first error.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Full migration from JSON storage to SQLite\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate all --config=/etc/lighthouse/config.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    --keys-source=/var/lib/lighthouse/legacy-keys \\\n")
		fmt.Fprintf(os.Stderr, "    --keys-dest=/var/lib/lighthouse/keys \\\n")
		fmt.Fprintf(os.Stderr, "    --data-source=/var/lib/lighthouse/legacy \\\n")
		fmt.Fprintf(os.Stderr, "    --source-type=json --db-type=sqlite --db-dir=/var/lib/lighthouse \\\n")
		fmt.Fprintf(os.Stderr, "    --algs=ES256,RS256 --pks-type=db\n\n")
		fmt.Fprintf(os.Stderr, "  # Migration to PostgreSQL, skipping key migrations\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate all --config=/etc/lighthouse/config.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    --data-source=/var/lib/lighthouse/badger --source-type=badger \\\n")
		fmt.Fprintf(os.Stderr, "    --db-type=postgres --db-dsn='host=localhost user=lh dbname=lighthouse' \\\n")
		fmt.Fprintf(os.Stderr, "    --skip=keys-public,keys-kms\n\n")
		fmt.Fprintf(os.Stderr, "  # Dry run to preview all changes\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate all --config=/etc/lighthouse/config.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    --keys-source=/var/lib/lighthouse/legacy-keys \\\n")
		fmt.Fprintf(os.Stderr, "    --keys-dest=/var/lib/lighthouse/keys \\\n")
		fmt.Fprintf(os.Stderr, "    --data-source=/var/lib/lighthouse/legacy \\\n")
		fmt.Fprintf(os.Stderr, "    --source-type=json --db-type=sqlite --db-dir=/var/lib/lighthouse \\\n")
		fmt.Fprintf(os.Stderr, "    --algs=ES256 --pks-type=db --dry-run -v\n")
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	// Parse skip steps
	skip, err := parseSkipSteps(skipSteps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 2
	}

	// Build the orchestrator
	orch := &allOrchestrator{
		configFile: configFile,
		destConfig: destConfig,
		keysSource: keysSource,
		keysDest:   keysDest,
		dataSource: dataSource,
		dataDest:   dataDest,
		sourceType: sourceType,
		dbType:     dbType,
		dbDSN:      dbDSN,
		dbDir:      dbDir,
		dbDebug:    dbDebug,
		keyType:    keyType,
		algsStr:    algsStr,
		pksType:    pksType,
		genKeys:    genKeys,
		rsaKeyLen:  rsaKeyLen,
		defAlg:     defAlg,
		skip:       skip,
		force:      force,
		dryRun:     dryRun,
		verbose:    verbose,
	}

	return orch.run()
}

// allOrchestrator manages the execution of all migration steps
type allOrchestrator struct {
	configFile string
	destConfig string
	keysSource string
	keysDest   string
	dataSource string
	dataDest   string
	sourceType string
	dbType     string
	dbDSN      string
	dbDir      string
	dbDebug    bool
	keyType    string
	algsStr    string
	pksType    string
	genKeys    bool
	rsaKeyLen  int
	defAlg     string
	skip       map[allStep]bool
	force      bool
	dryRun     bool
	verbose    bool
}

// shouldRun returns true if the step should be executed
func (o *allOrchestrator) shouldRun(step allStep) bool {
	return !o.skip[step]
}

// run executes all migration steps in order
func (o *allOrchestrator) run() int {
	steps := allSteps()
	totalSteps := 0
	for _, s := range steps {
		if o.shouldRun(s) {
			totalSteps++
		}
	}

	if totalSteps == 0 {
		fmt.Println("All steps skipped, nothing to do.")
		return 0
	}

	currentStep := 0
	for _, step := range steps {
		if !o.shouldRun(step) {
			log.WithField("step", string(step)).Debug("Skipping step")
			continue
		}

		currentStep++
		fmt.Printf("\n=== Step %d/%d: %s ===\n\n", currentStep, totalSteps, stepName(step))

		var code int
		switch step {
		case stepConfig:
			code = o.runConfigStep()
		case stepKeysPublic:
			code = o.runKeysPublicStep()
		case stepKeysKMS:
			code = o.runKeysKMSStep()
		case stepConfig2DB:
			code = o.runConfig2DBStep()
		case stepDB:
			code = o.runDBStep()
		case stepConfigCleanup:
			code = o.runConfigCleanupStep()
		}

		if code != 0 {
			fmt.Printf("\n=== Migration Failed at Step %d/%d: %s ===\n", currentStep, totalSteps, stepName(step))
			return code
		}
	}

	fmt.Println()
	fmt.Println("=== All Migrations Completed Successfully ===")
	return 0
}

// runConfigStep runs the config transformation step
func (o *allOrchestrator) runConfigStep() int {
	if o.configFile == "" {
		log.Warn("No config file specified, skipping config transformation")
		return 0
	}

	args := []string{"--source=" + o.configFile}

	// Determine destination config
	dest := o.destConfig
	if dest == "" {
		dest = o.configFile // Overwrite source
	}
	args = append(args, "--dest="+dest)

	// Add database options for storage section transformation
	if o.dbType != "" {
		args = append(args, "--db-type="+o.dbType)
	}
	if o.dbDSN != "" {
		args = append(args, "--db-dsn="+o.dbDSN)
	}
	if o.dbDir != "" {
		args = append(args, "--db-dir="+o.dbDir)
	}

	// Control flags
	if o.dryRun {
		args = append(args, "--dry-run")
	}
	if o.verbose {
		args = append(args, "-v")
	}

	log.WithField("args", args).Debug("Running config step")
	return runConfigMigration(args)
}

// runKeysPublicStep runs the public keys migration step
func (o *allOrchestrator) runKeysPublicStep() int {
	if o.keysSource == "" {
		log.Warn("No keys source directory specified (--keys-source), skipping public keys migration")
		return 0
	}

	// Destination
	dest := o.keysDest
	if dest == "" {
		dest = o.keysSource
	}

	// Build args with required options
	args := []string{
		"--source=" + o.keysSource,
		"--dest=" + dest,
		"--type=" + o.keyType,
	}

	// Database options for DB destination
	if o.pksType == "db" || o.dbType != "" {
		if o.dbType != "" {
			args = append(args, "--db-type="+o.dbType)
		}
		if o.dbDSN != "" {
			args = append(args, "--db-dsn="+o.dbDSN)
		}
		if o.dbDebug {
			args = append(args, "--db-debug")
		}
	}

	if o.verbose {
		args = append(args, "-v")
	}

	log.WithField("args", args).Debug("Running keys public step")
	return publicCmd(args)
}

// runKeysKMSStep runs the KMS migration step
func (o *allOrchestrator) runKeysKMSStep() int {
	if o.keysSource == "" {
		log.Warn("No keys source directory specified (--keys-source), skipping KMS migration")
		return 0
	}
	if o.algsStr == "" {
		log.Warn("No algorithms specified (--algs), skipping KMS migration")
		return 0
	}
	if o.pksType == "" {
		log.Warn("No public key storage type specified (--pks-type), skipping KMS migration")
		return 0
	}

	// Destination
	dest := o.keysDest
	if dest == "" {
		dest = o.keysSource
	}

	// Build args with required options
	args := []string{
		"--source=" + o.keysSource,
		"--dest=" + dest,
		"--type=" + o.keyType,
		"--algs=" + o.algsStr,
		"--pks-type=" + o.pksType,
		fmt.Sprintf("--rsa-len=%d", o.rsaKeyLen),
	}

	// Optional KMS settings
	if o.defAlg != "" {
		args = append(args, "--default="+o.defAlg)
	}
	if o.genKeys {
		args = append(args, "--generate-missing")
	}

	// Database options
	if o.pksType == "db" {
		if o.dbType != "" {
			args = append(args, "--db-type="+o.dbType)
		}
		if o.dbDSN != "" {
			args = append(args, "--db-dsn="+o.dbDSN)
		}
		if o.dbDebug {
			args = append(args, "--db-debug")
		}
	}

	if o.verbose {
		args = append(args, "-v")
	}

	log.WithField("args", args).Debug("Running keys kms step")
	return kmsCmd(args)
}

// runConfig2DBStep runs the config2db migration step
func (o *allOrchestrator) runConfig2DBStep() int {
	// Determine config file to use (could be transformed)
	configPath := o.destConfig
	if configPath == "" {
		configPath = o.configFile
	}
	if configPath == "" {
		log.Warn("No config file specified, skipping config2db migration")
		return 0
	}

	args := []string{"--config=" + configPath}

	// Database options
	if o.dbType != "" {
		args = append(args, "--db-type="+o.dbType)
	}
	if o.dbDSN != "" {
		args = append(args, "--db-dsn="+o.dbDSN)
	}
	if o.dbDir != "" {
		args = append(args, "--db-dir="+o.dbDir)
	}
	if o.dbDebug {
		args = append(args, "--db-debug")
	}

	// Enable update-config to remove migrated options
	args = append(args, "--update-config")

	// Control flags
	if o.force {
		args = append(args, "--force")
	}
	if o.dryRun {
		args = append(args, "--dry-run")
	}
	if o.verbose {
		args = append(args, "-v")
	}

	log.WithField("args", args).Debug("Running config2db step")
	return config2dbCmd(args)
}

// runDBStep runs the database migration step
func (o *allOrchestrator) runDBStep() int {
	if o.dataSource == "" {
		log.Warn("No data source directory specified (--data-source), skipping database migration")
		return 0
	}
	if o.sourceType == "" {
		log.Warn("No source type specified (--source-type), skipping database migration")
		return 0
	}

	args := []string{
		"--source-type=" + o.sourceType,
		"--source=" + o.dataSource,
	}

	// Database options
	if o.dbType != "" {
		args = append(args, "--db-type="+o.dbType)
	}

	// Destination based on database type
	if o.dbType == "sqlite" || o.dbType == "" {
		dest := o.dataDest
		if dest == "" {
			dest = o.dbDir
		}
		if dest != "" {
			args = append(args, "--dest="+dest)
		}
	}

	if o.dbDSN != "" {
		args = append(args, "--db-dsn="+o.dbDSN)
	}
	if o.dbDebug {
		args = append(args, "--db-debug")
	}

	// Control flags
	if o.force {
		args = append(args, "--force")
	}
	if o.dryRun {
		args = append(args, "--dry-run")
	}
	if o.verbose {
		args = append(args, "-v")
	}

	log.WithField("args", args).Debug("Running db step")
	return runDBMigration(args)
}

// runConfigCleanupStep runs the config cleanup step
func (o *allOrchestrator) runConfigCleanupStep() int {
	// Determine config file to clean up
	configPath := o.destConfig
	if configPath == "" {
		configPath = o.configFile
	}
	if configPath == "" {
		log.Warn("No config file specified, skipping config cleanup")
		return 0
	}

	if o.dryRun {
		fmt.Println("Would clean up empty values from config file:", configPath)
		return 0
	}

	fmt.Println("Cleaning up empty values from config file:", configPath)
	if err := cleanupConfigFile(configPath, o.verbose); err != nil {
		log.WithError(err).Error("Config cleanup failed")
		return 1
	}

	fmt.Println("Config file cleaned up successfully")
	return 0
}
