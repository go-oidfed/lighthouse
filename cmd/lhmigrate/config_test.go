package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMoveSigningFieldsToFilesystem(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected func(t *testing.T, output string)
	}{
		{
			name: "moves key_dir to filesystem subsection",
			input: `signing:
  key_dir: /path/to/keys
  auto_generate_keys: true
`,
			expected: func(t *testing.T, output string) {
				if !strings.Contains(output, "filesystem:") {
					t.Error("expected filesystem section to be created")
				}
				if !strings.Contains(output, "kms: filesystem") {
					t.Error("expected kms: filesystem to be added")
				}
				// key_dir should be under filesystem, not at signing level
				lines := strings.Split(output, "\n")
				inFilesystem := false
				for _, line := range lines {
					if strings.Contains(line, "filesystem:") {
						inFilesystem = true
					}
					if strings.Contains(line, "key_dir:") {
						if !inFilesystem {
							t.Error("key_dir should be under filesystem section")
						}
					}
				}
			},
		},
		{
			name: "moves key_file to filesystem subsection",
			input: `signing:
  key_file: /path/to/key.pem
  auto_generate_keys: true
`,
			expected: func(t *testing.T, output string) {
				if !strings.Contains(output, "filesystem:") {
					t.Error("expected filesystem section to be created")
				}
				if !strings.Contains(output, "kms: filesystem") {
					t.Error("expected kms: filesystem to be added")
				}
			},
		},
		{
			name: "moves both key_dir and key_file",
			input: `signing:
  key_dir: /path/to/keys
  key_file: /path/to/key.pem
  auto_generate_keys: true
`,
			expected: func(t *testing.T, output string) {
				if !strings.Contains(output, "filesystem:") {
					t.Error("expected filesystem section to be created")
				}
				if !strings.Contains(output, "key_dir:") {
					t.Error("expected key_dir to be present")
				}
				if !strings.Contains(output, "key_file:") {
					t.Error("expected key_file to be present")
				}
			},
		},
		{
			name: "appends to existing filesystem section",
			input: `signing:
  key_dir: /path/to/keys
  filesystem:
    some_option: value
`,
			expected: func(t *testing.T, output string) {
				if !strings.Contains(output, "some_option: value") {
					t.Error("expected existing filesystem options to be preserved")
				}
				if !strings.Contains(output, "key_dir:") {
					t.Error("expected key_dir to be present")
				}
			},
		},
		{
			name: "does not add kms if already present",
			input: `signing:
  kms: pkcs11
  key_dir: /path/to/keys
`,
			expected: func(t *testing.T, output string) {
				if strings.Contains(output, "kms: filesystem") {
					t.Error("should not override existing kms value")
				}
				if !strings.Contains(output, "kms: pkcs11") {
					t.Error("expected existing kms: pkcs11 to be preserved")
				}
			},
		},
		{
			name: "no changes if no key_dir or key_file",
			input: `signing:
  auto_generate_keys: true
`,
			expected: func(t *testing.T, output string) {
				if strings.Contains(output, "filesystem:") {
					t.Error("should not create filesystem section if no key_dir/key_file")
				}
				if strings.Contains(output, "kms:") {
					t.Error("should not add kms if no key_dir/key_file")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := &configTransformer{verbose: false}

			var root yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &root); err != nil {
				t.Fatalf("failed to parse input YAML: %v", err)
			}

			transformer.transformNode(&root)

			var buf strings.Builder
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(2)
			if err := encoder.Encode(&root); err != nil {
				t.Fatalf("failed to encode output YAML: %v", err)
			}
			encoder.Close()

			output := buf.String()
			tt.expected(t, output)
		})
	}
}

func TestTransformSigningNodeDeprecations(t *testing.T) {
	input := `signing:
  alg: ES512
  rsa_key_len: 4096
  key_dir: /path/to/keys
  automatic_key_rollover:
    enabled: true
    interval: 30d
`

	transformer := &configTransformer{verbose: false}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(input), &root); err != nil {
		t.Fatalf("failed to parse input YAML: %v", err)
	}

	transformer.transformNode(&root)

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		t.Fatalf("failed to encode output YAML: %v", err)
	}
	encoder.Close()

	output := buf.String()

	// Check automatic_key_rollover was renamed
	if strings.Contains(output, "automatic_key_rollover:") {
		t.Error("automatic_key_rollover should be renamed to key_rotation")
	}
	if !strings.Contains(output, "key_rotation:") {
		t.Error("expected key_rotation to be present (renamed from automatic_key_rollover)")
	}

	// Check deprecation comments
	if !strings.Contains(output, "DEPRECATED") {
		t.Error("expected DEPRECATED comments for alg, rsa_key_len, key_rotation")
	}

	// Check key_dir was moved to filesystem
	if !strings.Contains(output, "filesystem:") {
		t.Error("expected filesystem section to be created for key_dir")
	}
}
