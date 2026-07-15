package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var reader = bufio.NewReader(os.Stdin)

// promptString prompts for a string value. Pressing Enter keeps the current value.
func promptString(label, current string) string {
	if current != "" {
		fmt.Printf("%s [%s]: ", label, current)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return current
	}
	return input
}

// promptStringRequired prompts for a non-empty string.
func promptStringRequired(label string) string {
	for {
		fmt.Printf("%s: ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			return input
		}
		fmt.Println("  Value is required. Please try again.")
	}
}

// promptBool prompts for a yes/no value.
func promptBool(label string, current bool) bool {
	cur := "n"
	if current {
		cur = "y"
	}
	for {
		fmt.Printf("%s [y/n] (current: %s): ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" {
			return current
		}
		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Println("  Please enter 'y' or 'n'.")
	}
}

// promptInt prompts for an integer value.
func promptInt(label string, current int) int {
	cur := fmt.Sprintf("%d", current)
	for {
		fmt.Printf("%s [%s]: ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		v, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("  Invalid integer. Please try again.")
			continue
		}
		return v
	}
}

// promptInt64 prompts for an int64 value.
func promptInt64(label string, current int64) int64 {
	cur := fmt.Sprintf("%d", current)
	for {
		fmt.Printf("%s [%s]: ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		v, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			fmt.Println("  Invalid integer. Please try again.")
			continue
		}
		return v
	}
}

// promptFloat prompts for a float64 value.
func promptFloat(label string, current float64) float64 {
	cur := fmt.Sprintf("%g", current)
	for {
		fmt.Printf("%s [%s]: ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		v, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("  Invalid number. Please try again.")
			continue
		}
		return v
	}
}

// promptDuration prompts for a duration string (e.g. "24h", "30m", "600s").
func promptDuration(label string, current time.Duration) time.Duration {
	cur := current.String()
	for {
		fmt.Printf("%s (e.g. 24h, 30m, 600s) [%s]: ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		d, err := time.ParseDuration(input)
		if err != nil {
			fmt.Println("  Invalid duration. Please try again.")
			continue
		}
		return d
	}
}

// promptChoice prompts for a choice from a list of options.
func promptChoice(label string, options []string, current string) string {
	fmt.Printf("%s\n", label)
	for i, opt := range options {
		marker := ""
		if opt == current {
			marker = " (current)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, opt, marker)
	}
	for {
		fmt.Printf("Select [1-%d] (Enter to keep current): ", len(options))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(options) {
			fmt.Printf("  Please enter a number between 1 and %d.\n", len(options))
			continue
		}
		return options[idx-1]
	}
}

// promptFilePath prompts for a file path and validates the file exists.
func promptFilePath(label string) string {
	for {
		fmt.Printf("%s (file path, Enter to skip): ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return ""
		}
		if _, err := os.Stat(input); err != nil {
			fmt.Printf("  File not found: %s\n", err)
			continue
		}
		return input
	}
}

// promptConfirm asks for yes/no confirmation.
func promptConfirm(label string) bool {
	for {
		fmt.Printf("%s [y/n]: ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Println("  Please enter 'y' or 'n'.")
	}
}

// promptList prompts for a comma-separated list of strings.
func promptList(label string, current []string) []string {
	cur := strings.Join(current, ", ")
	for {
		fmt.Printf("%s (comma-separated) [%s]: ", label, cur)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return current
		}
		if input == "-" {
			return []string{}
		}
		parts := strings.Split(input, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
}

// printHeader prints a section header.
func printHeader(title string) {
	fmt.Println()
	fmt.Printf("=== %s ===\n", strings.ToUpper(title))
	fmt.Println()
}

// printSubHeader prints a subsection header.
func printSubHeader(title string) {
	fmt.Printf("\n--- %s ---\n", title)
}

// printValue prints a label-value pair.
func printValue(label string, value any) {
	fmt.Printf("  %s: %v\n", label, value)
}

// printList prints a label followed by a list of items.
func printList(label string, items []string) {
	if len(items) == 0 {
		fmt.Printf("  %s: (none)\n", label)
		return
	}
	fmt.Printf("  %s:\n", label)
	for _, item := range items {
		fmt.Printf("    - %s\n", item)
	}
}

// promptYesNo asks for yes/no confirmation without showing a "(current: ...)" suffix.
func promptYesNo(label string) bool {
	for {
		fmt.Printf("%s [y/n]: ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Println("  Please enter 'y' or 'n'.")
	}
}

// promptAdditionalClaimsMap interactively prompts for a map of claim name to
// JSON value. The user is asked to add claims one by one until they're done.
func promptAdditionalClaimsMap() map[string]any {
	claims := make(map[string]any)
	for {
		name := promptString("  Claim name (Enter to finish)", "")
		if name == "" {
			break
		}
		valueStr := promptStringRequired("  Claim value (JSON, e.g. \"string\", 123, true, {\"key\":\"val\"})")
		var value any
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			fmt.Printf("  Error parsing value as JSON: %s\n", err)
			continue
		}
		claims[name] = value
		if !promptYesNo("  Add another claim?") {
			break
		}
	}
	return claims
}

// promptTrustMarkType shows existing trust mark types from the DB as a
// numbered list, with a final option to enter a new type. Falls back to a
// plain text prompt if no types exist in the DB.
func promptTrustMarkType(label string) string {
	types, err := backends.TrustMarkTypes.List()
	if err != nil || len(types) == 0 {
		return promptStringRequired(label)
	}
	fmt.Printf("%s\n", label)
	for i, t := range types {
		fmt.Printf("  %d. %s\n", i+1, t.TrustMarkType)
	}
	fmt.Printf("  %d. Enter a new type\n", len(types)+1)
	for {
		fmt.Printf("Select [1-%d]: ", len(types)+1)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(types)+1 {
			fmt.Printf("  Please enter a number between 1 and %d.\n", len(types)+1)
			continue
		}
		if idx <= len(types) {
			return types[idx-1].TrustMarkType
		}
		return promptStringRequired("Enter trust mark type")
	}
}
