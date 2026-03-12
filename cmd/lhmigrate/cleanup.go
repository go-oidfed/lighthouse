package main

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/zachmann/go-utils/fileutils"
	"gopkg.in/yaml.v3"
)

// cleanupConfigFile removes empty values from the config file
// This includes empty arrays [], empty objects {}, empty strings "", and null values
func cleanupConfigFile(configPath string, verbose bool) error {
	content, err := fileutils.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Track what was removed for reporting
	var removed []string

	// Recursively clean empty values
	cleanEmptyValues(&root, "", &removed, verbose)

	// Marshal back to YAML
	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to close encoder: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(configPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Report what was removed
	if len(removed) > 0 {
		fmt.Printf("Removed %d empty values:\n", len(removed))
		for _, r := range removed {
			fmt.Printf("  - %s\n", r)
		}
	} else {
		fmt.Println("No empty values found to remove")
	}

	return nil
}

// cleanEmptyValues recursively removes empty values from a YAML node tree
// Returns true if the node itself is empty and should be removed
func cleanEmptyValues(node *yaml.Node, path string, removed *[]string, verbose bool) bool {
	if node == nil {
		return true
	}

	switch node.Kind {
	case yaml.DocumentNode:
		// Process document content
		for _, child := range node.Content {
			cleanEmptyValues(child, path, removed, verbose)
		}
		return false

	case yaml.MappingNode:
		return cleanMappingNode(node, path, removed, verbose)

	case yaml.SequenceNode:
		return cleanSequenceNode(node, path, removed, verbose)

	case yaml.ScalarNode:
		return isEmptyScalar(node)

	case yaml.AliasNode:
		// Don't clean aliases
		return false

	default:
		return false
	}
}

// cleanMappingNode cleans a mapping node and removes empty key-value pairs
// Returns true if the entire mapping is empty
func cleanMappingNode(node *yaml.Node, path string, removed *[]string, verbose bool) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}

	newContent := make([]*yaml.Node, 0, len(node.Content))

	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		keyPath := joinPath(path, keyNode.Value)

		// Recursively clean the value
		isEmpty := cleanEmptyValues(valueNode, keyPath, removed, verbose)

		if isEmpty {
			// Value is empty, remove this key-value pair
			*removed = append(*removed, keyPath)
			if verbose {
				log.WithField("path", keyPath).Debug("Removing empty value")
			}
		} else {
			// Keep this key-value pair
			newContent = append(newContent, keyNode, valueNode)
		}
	}

	node.Content = newContent

	// Return true if the mapping is now empty
	return len(node.Content) == 0
}

// cleanSequenceNode cleans a sequence node and removes empty elements
// Returns true if the entire sequence is empty
func cleanSequenceNode(node *yaml.Node, path string, removed *[]string, verbose bool) bool {
	if node.Kind != yaml.SequenceNode {
		return false
	}

	newContent := make([]*yaml.Node, 0, len(node.Content))

	for i, child := range node.Content {
		childPath := fmt.Sprintf("%s[%d]", path, i)

		// Recursively clean the child
		isEmpty := cleanEmptyValues(child, childPath, removed, verbose)

		if isEmpty {
			// Element is empty, remove it
			*removed = append(*removed, childPath)
			if verbose {
				log.WithField("path", childPath).Debug("Removing empty array element")
			}
		} else {
			// Keep this element
			newContent = append(newContent, child)
		}
	}

	node.Content = newContent

	// Return true if the sequence is now empty
	return len(node.Content) == 0
}

// isEmptyScalar checks if a scalar node is empty
func isEmptyScalar(node *yaml.Node) bool {
	if node.Kind != yaml.ScalarNode {
		return false
	}

	// Check for empty string
	if node.Value == "" {
		return true
	}

	// Check for null values
	if node.Tag == "!!null" {
		return true
	}
	if node.Value == "null" || node.Value == "~" {
		return true
	}

	return false
}

// joinPath joins path segments with a dot
func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}
