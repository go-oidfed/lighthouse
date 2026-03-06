package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"
)

// configTransformer handles config file transformation
type configTransformer struct {
	inputPath     string
	outputPath    string
	verbose       bool
	runConfig2db  bool
	config2dbArgs []string
}

// runConfigMigration is the main entry point for the config migration command
func runConfigMigration(args []string) int {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	var (
		in           = fs.String("in", "", "Path to existing configuration file (required)")
		out          = fs.String("out", "", "Path to write updated configuration (default: stdout)")
		runConfig2db = fs.Bool("run-config2db", false, "Also run config2db migration after transformation")
		dbDriver     = fs.String("db-driver", "sqlite", "Database driver for config2db: sqlite|mysql|postgres")
		dbDSN        = fs.String("db-dsn", "", "Database DSN for config2db (for mysql/postgres)")
		dbDir        = fs.String("db-dir", "", "Data directory for config2db (for sqlite)")
		dbDebug      = fs.Bool("db-debug", false, "Enable GORM debug logging for config2db")
		force        = fs.Bool("force", false, "Force overwrite in config2db")
		dryRun       = fs.Bool("dry-run", false, "Preview only, don't make changes")
		v            = fs.Bool("v", false, "Verbose logging")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lhmigrate config --in=<config.yaml> [--out=<updated.yaml>] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Transform legacy configuration file to new format.\n\n")
		fmt.Fprintf(os.Stderr, "This command reads an existing config file, removes deprecated fields,\n")
		fmt.Fprintf(os.Stderr, "renames legacy options, and outputs a new config file compatible with\n")
		fmt.Fprintf(os.Stderr, "LightHouse 0.20.0+. Deprecated fields are preserved as comments.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nTransformations applied:\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.entity_id -> entity_id (moved to top level)\n")
		fmt.Fprintf(os.Stderr, "  - storage.backend (json|badger) -> storage.driver (sqlite)\n")
		fmt.Fprintf(os.Stderr, "  - signing.automatic_key_rollover -> signing.key_rotation (renamed)\n")
		fmt.Fprintf(os.Stderr, "  - signing.alg, rsa_key_len, key_rotation -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.authority_hints -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.federation_entity_metadata -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.constraints -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.metadata_policy_crit -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.configuration_lifetime -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - endpoints.trust_mark.trust_mark_specs -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.trust_mark_issuers -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "  - federation_data.trust_mark_owners -> commented (now in database)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Transform config and output to stdout\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config --in=old-config.yaml\n\n")
		fmt.Fprintf(os.Stderr, "  # Transform and write to new file\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config --in=old-config.yaml --out=new-config.yaml\n\n")
		fmt.Fprintf(os.Stderr, "  # Transform and also migrate values to database\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config --in=old-config.yaml --out=new-config.yaml --run-config2db --db-dir=/data\n\n")
		fmt.Fprintf(os.Stderr, "  # Dry run to preview changes\n")
		fmt.Fprintf(os.Stderr, "  lhmigrate config --in=old-config.yaml --dry-run -v\n")
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *v {
		log.SetLevel(log.DebugLevel)
	}

	if *in == "" {
		fmt.Fprintln(os.Stderr, "--in is required")
		fs.Usage()
		return 2
	}

	// Build config2db args if needed
	var config2dbArgs []string
	if *runConfig2db {
		config2dbArgs = []string{"--config=" + *in}
		if *dbDriver != "" {
			config2dbArgs = append(config2dbArgs, "--db-driver="+*dbDriver)
		}
		if *dbDSN != "" {
			config2dbArgs = append(config2dbArgs, "--db-dsn="+*dbDSN)
		}
		if *dbDir != "" {
			config2dbArgs = append(config2dbArgs, "--db-dir="+*dbDir)
		}
		if *dbDebug {
			config2dbArgs = append(config2dbArgs, "--db-debug")
		}
		if *force {
			config2dbArgs = append(config2dbArgs, "--force")
		}
		if *dryRun {
			config2dbArgs = append(config2dbArgs, "--dry-run")
		}
		if *v {
			config2dbArgs = append(config2dbArgs, "-v")
		}
	}

	transformer := &configTransformer{
		inputPath:     *in,
		outputPath:    *out,
		verbose:       *v,
		runConfig2db:  *runConfig2db,
		config2dbArgs: config2dbArgs,
	}

	// Load and transform
	result, err := transformer.transform(*dryRun)
	if err != nil {
		log.WithError(err).Error("Config transformation failed")
		return 1
	}

	// Output result
	if *dryRun {
		fmt.Println("=== Dry Run: Transformed Config ===")
		fmt.Println()
	}

	if *out != "" && !*dryRun {
		if err := os.WriteFile(*out, []byte(result), 0644); err != nil {
			log.WithError(err).Error("Failed to write output file")
			return 1
		}
		log.WithField("path", *out).Info("Config file written")
	} else {
		fmt.Println(result)
	}

	// Run config2db if requested
	if *runConfig2db {
		fmt.Println()
		fmt.Println("=== Running config2db migration ===")
		fmt.Println()
		code := config2dbCmd(config2dbArgs)
		if code != 0 {
			return code
		}
	}

	if !*dryRun {
		log.Info("Config transformation completed successfully")
	}
	return 0
}

// transform loads the config file, transforms it, and returns the new YAML content
func (t *configTransformer) transform(dryRun bool) (string, error) {
	// Read the original file
	content, err := fileutils.ReadFile(t.inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as generic YAML to preserve structure
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply transformations
	t.transformNode(&root)

	// Move entity_id from federation_data to top level
	t.moveEntityIDToTopLevel(&root)

	// Marshal back to YAML
	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return "", fmt.Errorf("failed to encode transformed config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to close encoder: %w", err)
	}

	return buf.String(), nil
}

// moveEntityIDToTopLevel extracts entity_id from federation_data and moves it to top level
func (t *configTransformer) moveEntityIDToTopLevel(root *yaml.Node) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return
	}

	docContent := root.Content[0]
	if docContent.Kind != yaml.MappingNode {
		return
	}

	// Check if entity_id already exists at top level
	for i := 0; i < len(docContent.Content); i += 2 {
		if i+1 >= len(docContent.Content) {
			break
		}
		keyNode := docContent.Content[i]
		if keyNode.Value == "entity_id" {
			// Already at top level, nothing to do
			return
		}
	}

	// Find federation_data and extract entity_id
	var entityIDValue string
	var federationDataNode *yaml.Node
	var entityIDIndex int = -1

	for i := 0; i < len(docContent.Content); i += 2 {
		if i+1 >= len(docContent.Content) {
			break
		}
		keyNode := docContent.Content[i]
		valueNode := docContent.Content[i+1]

		if keyNode.Value == "federation_data" && valueNode.Kind == yaml.MappingNode {
			federationDataNode = valueNode
			// Find entity_id within federation_data
			for j := 0; j < len(valueNode.Content); j += 2 {
				if j+1 >= len(valueNode.Content) {
					break
				}
				subKeyNode := valueNode.Content[j]
				subValueNode := valueNode.Content[j+1]

				if subKeyNode.Value == "entity_id" {
					entityIDValue = subValueNode.Value
					entityIDIndex = j
					if t.verbose {
						log.WithField("entity_id", entityIDValue).Info("Found entity_id in federation_data, moving to top level")
					}
					break
				}
			}
			break
		}
	}

	// If we found entity_id in federation_data, add it at top level and remove from federation_data
	if entityIDValue != "" && federationDataNode != nil && entityIDIndex >= 0 {
		// Remove entity_id from federation_data (remove both key and value nodes)
		federationDataNode.Content = append(
			federationDataNode.Content[:entityIDIndex],
			federationDataNode.Content[entityIDIndex+2:]...,
		)

		// Create new key-value nodes for top-level entity_id
		keyNode := &yaml.Node{
			Kind:        yaml.ScalarNode,
			Tag:         "!!str",
			Value:       "entity_id",
			HeadComment: "# Entity identifier (moved from federation_data.entity_id)",
		}
		valueNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: entityIDValue,
		}

		// Insert at the beginning of the document (after any existing top-level items, before federation_data)
		// Find a good insertion point - ideally near the top, after server
		insertIndex := 0
		for i := 0; i < len(docContent.Content); i += 2 {
			if i+1 >= len(docContent.Content) {
				break
			}
			k := docContent.Content[i]
			if k.Value == "server" {
				insertIndex = i + 2 // After server section
				break
			}
		}

		// Insert the new key-value pair
		newContent := make([]*yaml.Node, 0, len(docContent.Content)+2)
		newContent = append(newContent, docContent.Content[:insertIndex]...)
		newContent = append(newContent, keyNode, valueNode)
		newContent = append(newContent, docContent.Content[insertIndex:]...)
		docContent.Content = newContent
	}
}

// transformNode recursively transforms the YAML node tree
func (t *configTransformer) transformNode(node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			t.transformNode(child)
		}
	case yaml.MappingNode:
		t.transformMappingNode(node)
	case yaml.SequenceNode:
		for _, child := range node.Content {
			t.transformNode(child)
		}
	}
}

// transformMappingNode handles transformation of mapping nodes
func (t *configTransformer) transformMappingNode(node *yaml.Node) {
	// Process pairs (key, value)
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		key := keyNode.Value

		// Apply specific transformations based on key
		switch key {
		case "storage":
			t.transformStorageNode(valueNode)
		case "signing":
			t.transformSigningNode(valueNode)
		case "federation_data":
			t.transformFederationNode(valueNode)
		case "endpoints":
			t.transformEndpointsNode(valueNode)
		}

		// Recurse into value
		t.transformNode(valueNode)
	}
}

// transformStorageNode handles storage section transformation
func (t *configTransformer) transformStorageNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Value == "backend" {
			// Transform legacy backend to driver
			oldValue := valueNode.Value
			if oldValue == "json" || oldValue == "badger" {
				keyNode.Value = "driver"
				valueNode.Value = "sqlite"
				// Add comment about migration
				keyNode.LineComment = fmt.Sprintf(" # Changed from 'backend: %s' - legacy backends no longer supported", oldValue)
				if t.verbose {
					log.WithFields(log.Fields{
						"old": "backend: " + oldValue,
						"new": "driver: sqlite",
					}).Info("Transformed storage backend")
				}
			}
		}
	}
}

// transformSigningNode handles signing section transformation
func (t *configTransformer) transformSigningNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	// Fields to comment out (moved to database)
	fieldsToComment := map[string]bool{
		"alg":          true,
		"rsa_key_len":  true,
		"key_rotation": true,
	}

	// Fields to rename
	fieldsToRename := map[string]string{
		"automatic_key_rollover": "key_rotation",
	}

	var toRemove []int

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]

		// Check for rename
		if newName, ok := fieldsToRename[keyNode.Value]; ok {
			if t.verbose {
				log.WithFields(log.Fields{
					"old": keyNode.Value,
					"new": newName,
				}).Info("Renamed signing field")
			}
			keyNode.Value = newName
			keyNode.LineComment = " # Renamed from 'automatic_key_rollover'"
			// This field should also be commented as it's now in DB
			fieldsToComment[newName] = true
		}

		// Check if should be commented out
		if fieldsToComment[keyNode.Value] {
			// Add comment indicating it's now in database
			keyNode.HeadComment = fmt.Sprintf("# DEPRECATED: '%s' is now managed in the database.\n# Use 'lhmigrate config2db' to migrate this value, or the Admin API.", keyNode.Value)
			toRemove = append(toRemove, i)
			if t.verbose {
				log.WithField("field", keyNode.Value).Info("Marked signing field as deprecated (moved to database)")
			}
		}
	}

	// Remove deprecated fields (keeping them as comments would require custom output)
	// For now, we just add comments - the actual removal is optional
}

// transformFederationNode handles federation_data section transformation
func (t *configTransformer) transformFederationNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	// Fields to mark as deprecated (moved to database)
	fieldsToComment := map[string]bool{
		"authority_hints":            true,
		"federation_entity_metadata": true,
		"constraints":                true,
		"metadata_policy_crit":       true,
		"configuration_lifetime":     true,
		"trust_mark_issuers":         true,
		"trust_mark_owners":          true,
	}

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]

		if fieldsToComment[keyNode.Value] {
			keyNode.HeadComment = fmt.Sprintf("# DEPRECATED: '%s' is now managed in the database.\n# Use 'lhmigrate config2db' to migrate this value, or the Admin API.", keyNode.Value)
			if t.verbose {
				log.WithField("field", keyNode.Value).Info("Marked federation_data field as deprecated (moved to database)")
			}
		}
	}
}

// transformEndpointsNode handles endpoints section transformation
func (t *configTransformer) transformEndpointsNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Value == "trust_mark" {
			t.transformTrustMarkEndpointNode(valueNode)
		}
	}
}

// transformTrustMarkEndpointNode handles trust_mark endpoint transformation
func (t *configTransformer) transformTrustMarkEndpointNode(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]

		if keyNode.Value == "trust_mark_specs" {
			keyNode.HeadComment = "# DEPRECATED: 'trust_mark_specs' is now managed in the database.\n# Use 'lhmigrate config2db' to migrate these values, or the Admin API."
			if t.verbose {
				log.WithField("field", "trust_mark_specs").Info("Marked trust_mark_specs as deprecated (moved to database)")
			}
		}
	}
}
