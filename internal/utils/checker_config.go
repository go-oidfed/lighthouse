package utils

// NormalizeTrustAnchors recursively walks a checker config value and transforms
// any trust_anchors field from inline TrustAnchor objects ({"entity_id": ...})
// into plain entity-id string references. Bare strings are left untouched.
//
// This lets DB-backed checker configs accept both the legacy YAML object form
// and the entity-id string form used by the database.
func NormalizeTrustAnchors(v any) any {
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
			val[k] = NormalizeTrustAnchors(item)
		}
		return val
	case []any:
		for i, item := range val {
			val[i] = NormalizeTrustAnchors(item)
		}
		return val
	default:
		return v
	}
}
