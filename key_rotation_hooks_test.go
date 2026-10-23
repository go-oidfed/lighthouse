package lighthouse

import (
	"testing"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func TestBuildAuthorityHintJWKSSyncHooks(t *testing.T) {
	entityID := "https://lh.example.com"

	t.Run("NoHints", func(t *testing.T) {
		hooks, err := buildAuthorityHintJWKSSyncHooks(entityID, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hooks) != 0 {
			t.Fatalf("expected 0 hooks, got %d", len(hooks))
		}
	})

	t.Run("Skipped", func(t *testing.T) {
		hints := []model.AuthorityHint{
			{EntityID: "https://a.example"},                                                          // nil jwks_sync
			{EntityID: "https://b.example", JWKSSync: &model.JWKSSyncConf{Mode: ""}},                 // empty
			{EntityID: "https://c.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncNone}}, // none
		}
		hooks, err := buildAuthorityHintJWKSSyncHooks(entityID, nil, hints)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hooks) != 0 {
			t.Fatalf("expected 0 hooks, got %d", len(hooks))
		}
	})

	t.Run("TriggerProducesHook", func(t *testing.T) {
		hints := []model.AuthorityHint{
			{EntityID: "https://ta.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncTrigger}},
		}
		hooks, err := buildAuthorityHintJWKSSyncHooks(entityID, nil, hints)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hooks) != 1 {
			t.Fatalf("expected 1 hook, got %d", len(hooks))
		}
	})

	t.Run("PushRequiresSigner", func(t *testing.T) {
		hints := []model.AuthorityHint{
			{EntityID: "https://ta.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncPush}},
		}
		_, err := buildAuthorityHintJWKSSyncHooks(entityID, nil, hints)
		if err == nil {
			t.Fatal("expected error for push mode without a signer")
		}
	})

	t.Run("MultipleHints", func(t *testing.T) {
		// Two trigger hints share one ROProducer; both should yield hooks.
		hints := []model.AuthorityHint{
			{EntityID: "https://ta1.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncTrigger}},
			{EntityID: "https://ta2.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncTrigger}},
			{EntityID: "https://none.example", JWKSSync: &model.JWKSSyncConf{Mode: model.JWKSSyncNone}},
		}
		hooks, err := buildAuthorityHintJWKSSyncHooks(entityID, nil, hints)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hooks) != 2 {
			t.Fatalf("expected 2 hooks, got %d", len(hooks))
		}
	})
}
