package migration

import "slices"

// Section represents which sections to migrate/configure
type Section string

const (
	SectionSigning                     Section = "signing"
	SectionFederation                  Section = "federation"
	SectionTrustMarkSpecs              Section = "trust_mark_specs"
	SectionTrustMarks                  Section = "trust_marks"
	SectionAuthorityHints              Section = "authority_hints"
	SectionMetadata                    Section = "metadata"
	SectionConstraints                 Section = "constraints"
	SectionMetadataPolicyCrit          Section = "metadata_policy_crit"
	SectionMetadataPolicies            Section = "metadata_policies"
	SectionConfigLifetime              Section = "config_lifetime"
	SectionStatementLifetime           Section = "statement_lifetime"
	SectionAlg                         Section = "alg"
	SectionRSAKeyLen                   Section = "rsa_key_len"
	SectionKeyRotation                 Section = "key_rotation"
	SectionTrustMarkIssuers            Section = "trust_mark_issuers"
	SectionTrustMarkOwners             Section = "trust_mark_owners"
	SectionExtraEntityConfigData       Section = "extra_entity_config"
	SectionSubordinateAdditionalClaims Section = "subordinate_additional_claims"
	SectionTrustAnchors                Section = "trust_anchors"
	SectionEndpoints                   Section = "endpoints"
)

// AllSections returns all available migration/config sections
func AllSections() []Section {
	return []Section{
		SectionAlg,
		SectionRSAKeyLen,
		SectionKeyRotation,
		SectionConstraints,
		SectionMetadataPolicyCrit,
		SectionMetadataPolicies,
		SectionConfigLifetime,
		SectionStatementLifetime,
		SectionAuthorityHints,
		SectionMetadata,
		SectionExtraEntityConfigData,
		SectionSubordinateAdditionalClaims,
		SectionTrustMarkSpecs,
		SectionTrustMarks,
		SectionTrustMarkIssuers,
		SectionTrustMarkOwners,
		SectionTrustAnchors,
		SectionEndpoints,
	}
}

// ParseSections parses a comma-separated list of sections
func ParseSections(s string) ([]Section, error) {
	if s == "" || s == "all" {
		return AllSections(), nil
	}

	parts := splitAndTrim(s, ",")
	sections := make([]Section, 0, len(parts))

	for _, p := range parts {
		sec := Section(p)
		if !IsValidSection(sec) {
			return nil, &InvalidSectionError{Section: p}
		}
		sections = append(sections, sec)
	}
	return sections, nil
}

// ParseSkipSections parses sections to skip
func ParseSkipSections(s string) (map[Section]bool, error) {
	if s == "" {
		return nil, nil
	}

	parts := splitAndTrim(s, ",")
	skip := make(map[Section]bool, len(parts))

	for _, p := range parts {
		sec := Section(p)
		if !IsValidSection(sec) {
			return nil, &InvalidSectionError{Section: p}
		}
		skip[sec] = true
	}
	return skip, nil
}

// IsValidSection checks if a section is valid
func IsValidSection(s Section) bool {
	return slices.Contains(AllSections(), s)
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
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

// InvalidSectionError is returned when an invalid section is specified
type InvalidSectionError struct {
	Section string
}

func (e *InvalidSectionError) Error() string {
	return "invalid section: " + e.Section
}
