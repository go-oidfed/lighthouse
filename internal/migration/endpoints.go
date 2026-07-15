package migration

import (
	"encoding/json"
	"os"

	"github.com/go-oidfed/lib/jwx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// ResolveDBConfigJSON is the JSON config struct for the resolve endpoint's
// type-specific config stored in the federation_endpoints table.
type ResolveDBConfigJSON struct {
	AllowedTrustAnchors                    []string                       `json:"allowed_trust_anchors,omitempty"`
	UseEntityCollectionAllowedTrustAnchors bool                           `json:"use_entity_collection_allowed_trust_anchors,omitempty"`
	GracePeriodSeconds                     int64                          `json:"grace_period_seconds,omitempty"`
	TimeElapsedGraceFactor                 float64                        `json:"time_elapsed_grace_factor,omitempty"`
	ProactiveResolver                      *ProactiveResolverDBConfigJSON `json:"proactive_resolver,omitempty"`
}

// ProactiveResolverDBConfigJSON is the JSON config struct for the proactive
// resolver sub-config of the resolve endpoint.
type ProactiveResolverDBConfigJSON struct {
	Enabled                  bool   `json:"enabled"`
	ConcurrencyLimit         int    `json:"concurrency_limit,omitempty"`
	QueueSize                int    `json:"queue_size,omitempty"`
	ResponseStorageDir       string `json:"response_storage_dir,omitempty"`
	ResponseStorageStoreJSON bool   `json:"response_storage_store_json,omitempty"`
	ResponseStorageStoreJWT  bool   `json:"response_storage_store_jwt,omitempty"`
}

// EnrollDBConfigJSON is the JSON config struct for the enroll endpoint's
// type-specific config.
type EnrollDBConfigJSON struct {
	CheckerType   string `json:"checker_type,omitempty"`
	CheckerConfig any    `json:"checker_config,omitempty"`
}

// CollectionDBConfigJSON is the JSON config struct for the entity collection
// endpoint's type-specific config.
type CollectionDBConfigJSON struct {
	AllowedTrustAnchors []string `json:"allowed_trust_anchors,omitempty"`
	IntervalSeconds     int64    `json:"interval_seconds,omitempty"`
	ConcurrencyLimit    int      `json:"concurrency_limit,omitempty"`
	PaginationLimit     int      `json:"pagination_limit,omitempty"`
}

// StrPtrOrNil returns a *string for a non-empty s, or nil for empty.
func StrPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ExtractCheckerTrustAnchorIDs walks a checker config YAML node and extracts
// all trust_anchor entity IDs found in trust_anchors fields.
func ExtractCheckerTrustAnchorIDs(node *yaml.Node) []string {
	var ids []string
	if node == nil || node.Kind == 0 {
		return ids
	}
	walkYAMLForKey(node, "trust_anchors", func(n *yaml.Node) {
		if n.Kind == yaml.SequenceNode {
			for _, item := range n.Content {
				if item.Kind == yaml.ScalarNode {
					ids = append(ids, item.Value)
				} else if item.Kind == yaml.MappingNode {
					for i := 0; i < len(item.Content); i += 2 {
						if item.Content[i].Value == "entity_id" {
							ids = append(ids, item.Content[i+1].Value)
							break
						}
					}
				}
			}
		}
	})
	return ids
}

func walkYAMLForKey(node *yaml.Node, key string, fn func(*yaml.Node)) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, c := range node.Content {
			walkYAMLForKey(c, key, fn)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				fn(node.Content[i+1])
			}
			walkYAMLForKey(node.Content[i+1], key, fn)
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			walkYAMLForKey(c, key, fn)
		}
	}
}

// ConvertCheckerConfig converts a checker config YAML node to a (type, config)
// pair suitable for JSON storage, transforming any trust_anchors from inline
// TrustAnchor objects to []string entity-id references.
func ConvertCheckerConfig(node *yaml.Node) (string, any) {
	if node == nil || node.Kind == 0 {
		return "", nil
	}

	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		log.WithError(err).Warn("failed to decode checker config for migration")
		return "", nil
	}

	checkerType, _ := raw["type"].(string)
	delete(raw, "type")

	var configVal any
	if c, ok := raw["config"]; ok {
		configVal = c
	}

	transformTrustAnchorsInValue(configVal)

	return checkerType, configVal
}

func transformTrustAnchorsInValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, item := range val {
			if k == "trust_anchors" {
				if arr, ok := item.([]any); ok {
					var ids []string
					for _, entry := range arr {
						if s, ok := entry.(string); ok {
							ids = append(ids, s)
						} else if mp, ok := entry.(map[string]any); ok {
							if eid, ok := mp["entity_id"].(string); ok {
								ids = append(ids, eid)
							}
						}
					}
					if len(ids) > 0 {
						val[k] = ids
					}
				}
				continue
			}
			transformTrustAnchorsInValue(item)
		}
	case []any:
		for _, item := range val {
			transformTrustAnchorsInValue(item)
		}
	}
}

// ParseJWKSFile reads and parses a JWKS file into a model.JWKS.
func ParseJWKSFile(path string) (*model.JWKS, error) {
	jwksData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed jwx.JWKS
	if err := parsed.UnmarshalJSON(jwksData); err != nil {
		return nil, err
	}
	return &model.JWKS{Keys: parsed}, nil
}

// ParseJWKSFromAny marshals an arbitrary value to JSON and parses it as a JWKS.
func ParseJWKSFromAny(raw any) (*model.JWKS, error) {
	jwksBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var parsed jwx.JWKS
	if err := parsed.UnmarshalJSON(jwksBytes); err != nil {
		return nil, err
	}
	if parsed.Set == nil {
		return nil, nil
	}
	return &model.JWKS{Keys: parsed}, nil
}
