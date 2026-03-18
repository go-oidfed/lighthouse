package adminapi

import (
	"encoding/json"
	"testing"
)

func TestYamlToJSON(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantErr  bool
		validate func(t *testing.T, result []byte)
	}{
		{
			name: "simple object",
			yamlData: `
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
`,
			validate: func(t *testing.T, result []byte) {
				var m map[string]any
				if err := json.Unmarshal(result, &m); err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}
				if m["openapi"] != "3.0.0" {
					t.Errorf("Expected openapi=3.0.0, got %v", m["openapi"])
				}
				info, ok := m["info"].(map[string]any)
				if !ok {
					t.Fatal("Expected info to be a map")
				}
				if info["title"] != "Test API" {
					t.Errorf("Expected title='Test API', got %v", info["title"])
				}
			},
		},
		{
			name: "servers array",
			yamlData: `
servers:
  - url: http://localhost:8080
    description: Local server
  - url: https://api.example.com
    description: Production
`,
			validate: func(t *testing.T, result []byte) {
				var m map[string]any
				if err := json.Unmarshal(result, &m); err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}
				servers, ok := m["servers"].([]any)
				if !ok {
					t.Fatal("Expected servers to be an array")
				}
				if len(servers) != 2 {
					t.Errorf("Expected 2 servers, got %d", len(servers))
				}
				server1, ok := servers[0].(map[string]any)
				if !ok {
					t.Fatal("Expected server[0] to be a map")
				}
				if server1["url"] != "http://localhost:8080" {
					t.Errorf("Expected url='http://localhost:8080', got %v", server1["url"])
				}
			},
		},
		{
			name:     "invalid YAML",
			yamlData: "{{invalid yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yamlToJSON([]byte(tt.yamlData))
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertMapKeysToStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		verify func(t *testing.T, result any)
	}{
		{
			name: "map[string]any passthrough",
			input: map[string]any{
				"key": "value",
			},
			verify: func(t *testing.T, result any) {
				m, ok := result.(map[string]any)
				if !ok {
					t.Fatal("Expected map[string]any")
				}
				if m["key"] != "value" {
					t.Errorf("Expected key=value, got %v", m["key"])
				}
			},
		},
		{
			name: "nested maps",
			input: map[string]any{
				"outer": map[string]any{
					"inner": "value",
				},
			},
			verify: func(t *testing.T, result any) {
				m, ok := result.(map[string]any)
				if !ok {
					t.Fatal("Expected map[string]any")
				}
				outer, ok := m["outer"].(map[string]any)
				if !ok {
					t.Fatal("Expected outer to be map[string]any")
				}
				if outer["inner"] != "value" {
					t.Errorf("Expected inner=value, got %v", outer["inner"])
				}
			},
		},
		{
			name: "array with maps",
			input: []any{
				map[string]any{"a": 1},
				map[string]any{"b": 2},
			},
			verify: func(t *testing.T, result any) {
				arr, ok := result.([]any)
				if !ok {
					t.Fatal("Expected []any")
				}
				if len(arr) != 2 {
					t.Errorf("Expected 2 elements, got %d", len(arr))
				}
			},
		},
		{
			name:  "primitive passthrough",
			input: "hello",
			verify: func(t *testing.T, result any) {
				if result != "hello" {
					t.Errorf("Expected 'hello', got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMapKeysToStrings(tt.input)
			tt.verify(t, result)
		})
	}
}
