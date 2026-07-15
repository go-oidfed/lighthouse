package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse/storage"
)

func sectionConfigLifetime() {
	printHeader("Entity Configuration Lifetime")

	current, err := storage.GetEntityConfigurationLifetime(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}
	printValue("Current", current)

	var hint time.Duration
	if migrationCfg != nil && migrationCfg.Federation.ConfigurationLifetime.Duration() > 0 {
		hint = migrationCfg.Federation.ConfigurationLifetime.Duration()
		if hint != current {
			printValue("Config file hint", hint)
		}
	}

	if !promptConfirm("Change?") {
		return
	}

	def := current
	if hint > 0 {
		def = hint
	}
	newVal := promptDuration("New lifetime", def)
	if newVal == current {
		fmt.Println("  No change.")
		return
	}
	if err := storage.SetEntityConfigurationLifetime(backends.KV, newVal); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Set to %s\n", newVal)
}

func sectionStatementLifetime() {
	printHeader("Subordinate Statement Lifetime")

	current, err := storage.GetSubordinateStatementLifetime(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}
	printValue("Current", current)

	var hint time.Duration
	if migrationCfg != nil && migrationCfg.Endpoints.Fetch.StatementLifetime.Duration() > 0 {
		hint = migrationCfg.Endpoints.Fetch.StatementLifetime.Duration()
		if hint != current {
			printValue("Config file hint", hint)
		}
	}

	if !promptConfirm("Change?") {
		return
	}

	def := current
	if hint > 0 {
		def = hint
	}
	newVal := promptDuration("New lifetime", def)
	if newVal == current {
		fmt.Println("  No change.")
		return
	}
	seconds := int(newVal.Seconds())
	if err := backends.KV.SetAny(
		"subordinate_statement", "lifetime", seconds,
	); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Set to %s\n", newVal)
}

func sectionSigningAlg() {
	printHeader("Signing Algorithm")

	current, err := storage.GetSigningAlg(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}
	printValue("Current", current.String())

	supported := jwx.SupportedAlgsStrings()

	var hint string
	if migrationCfg != nil && migrationCfg.Signing.Alg != "" {
		hint = migrationCfg.Signing.Alg
		if hint != current.String() {
			printValue("Config file hint", hint)
		}
	}

	if !promptConfirm("Change?") {
		return
	}

	choice := promptChoice("Select signing algorithm:", supported, current.String())
	if choice == current.String() {
		fmt.Println("  No change.")
		return
	}

	alg, found := jwa.LookupSignatureAlgorithm(choice)
	if !found {
		fmt.Printf("  Invalid algorithm: %s\n", choice)
		return
	}

	if err := storage.SetSigningAlg(backends.KV, storage.SigningAlgWithNbf{
		SigningAlg: alg.String(),
	}); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Set to %s\n", alg.String())
}

func isRSAAlg(alg string) bool {
	a := strings.ToUpper(alg)
	return strings.HasPrefix(a, "RS") || strings.HasPrefix(a, "PS")
}

func sectionRSAKeyLen() {
	printHeader("RSA Key Length")

	current, err := storage.GetRSAKeyLen(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}

	alg, err := storage.GetSigningAlg(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading signing algorithm: %s\n", err)
		return
	}

	if !isRSAAlg(alg.String()) {
		fmt.Printf("  Current signing algorithm is %s (non-RSA). RSA key length is not needed.\n", alg.String())
		return
	}

	printValue("Current", current)

	var hint int
	if migrationCfg != nil && migrationCfg.Signing.RSAKeyLen > 0 {
		hint = migrationCfg.Signing.RSAKeyLen
		if hint != current {
			printValue("Config file hint", hint)
		}
	}

	if !promptConfirm("Change?") {
		return
	}

	def := current
	if hint > 0 {
		def = hint
	}
	newVal := promptInt("New RSA key length (minimum 2048)", def)
	if newVal < 2048 {
		fmt.Println("  RSA key length must be at least 2048.")
		return
	}
	if newVal == current {
		fmt.Println("  No change.")
		return
	}
	if err := storage.SetRSAKeyLen(backends.KV, newVal); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Printf("  Set to %d\n", newVal)
}

func sectionKeyRotation() {
	printHeader("Key Rotation")

	current, err := storage.GetKeyRotation(backends.KV)
	if err != nil {
		fmt.Printf("  Error reading current value: %s\n", err)
		return
	}
	printValue("Enabled", current.Enabled)
	printValue("Interval", time.Duration(current.Interval.Duration()))
	printValue("Overlap", time.Duration(current.Overlap.Duration()))
	printValue("Key announcement lead time", time.Duration(current.KeyAnnouncementLeadTime.Duration()))
	printValue("Key announcement lead time EC multiplier", current.KeyAnnouncementLeadTimeECMultiplier)

	var hintConfig *kms.KeyRotationConfig
	if migrationCfg != nil {
		if migrationCfg.Signing.KeyRotation.Interval.Duration() > 0 {
			hintConfig = &kms.KeyRotationConfig{
				Enabled:                             migrationCfg.Signing.KeyRotation.Enabled,
				Interval:                            migrationCfg.Signing.KeyRotation.Interval,
				Overlap:                             migrationCfg.Signing.KeyRotation.Overlap,
				KeyAnnouncementLeadTime:             migrationCfg.Signing.KeyRotation.KeyAnnouncementLeadTime,
				KeyAnnouncementLeadTimeECMultiplier: migrationCfg.Signing.KeyRotation.KeyAnnouncementLeadTimeECMultiplier,
			}
		} else if migrationCfg.Signing.AutomaticKeyRollover.Interval.Duration() > 0 {
			hintConfig = &kms.KeyRotationConfig{
				Enabled:  migrationCfg.Signing.AutomaticKeyRollover.Enabled,
				Interval: migrationCfg.Signing.AutomaticKeyRollover.Interval,
			}
		}
	}
	if hintConfig != nil {
		fmt.Println("  Config file hint:")
		printValue("    Enabled", hintConfig.Enabled)
		printValue("    Interval", time.Duration(hintConfig.Interval.Duration()))
		if hintConfig.Overlap.Duration() > 0 {
			printValue("    Overlap", time.Duration(hintConfig.Overlap.Duration()))
		}
	}

	if !promptConfirm("Change?") {
		return
	}

	cfg := current

	defEnabled := cfg.Enabled
	if hintConfig != nil {
		defEnabled = hintConfig.Enabled
	}
	cfg.Enabled = promptBool("Enabled?", defEnabled)

	defInterval := time.Duration(cfg.Interval.Duration())
	if defInterval == 0 {
		defInterval = 600000 * time.Second
	}
	if hintConfig != nil && hintConfig.Interval.Duration() > 0 {
		defInterval = time.Duration(hintConfig.Interval.Duration())
	}
	cfg.Interval = duration.DurationOption(promptDuration("Interval", defInterval))

	defOverlap := time.Duration(cfg.Overlap.Duration())
	if defOverlap == 0 {
		defOverlap = time.Hour
	}
	if hintConfig != nil && hintConfig.Overlap.Duration() > 0 {
		defOverlap = time.Duration(hintConfig.Overlap.Duration())
	}
	cfg.Overlap = duration.DurationOption(promptDuration("Overlap", defOverlap))

	defLeadTime := time.Duration(cfg.KeyAnnouncementLeadTime.Duration())
	if hintConfig != nil && hintConfig.KeyAnnouncementLeadTime.Duration() > 0 {
		defLeadTime = time.Duration(hintConfig.KeyAnnouncementLeadTime.Duration())
	}
	cfg.KeyAnnouncementLeadTime = duration.DurationOption(promptDuration("Key announcement lead time (0 = use default/multiplier)", defLeadTime))

	defMultiplier := cfg.KeyAnnouncementLeadTimeECMultiplier
	if hintConfig != nil && hintConfig.KeyAnnouncementLeadTimeECMultiplier > 0 {
		defMultiplier = hintConfig.KeyAnnouncementLeadTimeECMultiplier
	}
	cfg.KeyAnnouncementLeadTimeECMultiplier = promptFloat("Key announcement lead time EC multiplier (0 = unused)", defMultiplier)

	if err := storage.SetKeyRotation(backends.KV, cfg); err != nil {
		fmt.Printf("  Error writing: %s\n", err)
		return
	}
	fmt.Println("  Key rotation config saved.")
}
