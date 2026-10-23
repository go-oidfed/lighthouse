package lighthouse

import (
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// triggerAssertionLifetime is the lifetime of the private_key_jwt client
// assertion used to authenticate to a trigger endpoint. Not configurable.
const triggerAssertionLifetime = time.Minute

// applyAuthorityHintJWKSSyncHooks rebuilds the outbound JWKS-sync key rotation
// hooks from the current authority-hint state and attaches them to the KMS
// rotation config. It is invoked at boot (after the signer exists) and after
// every authority-hint mutation (create/update/delete) so that jwks_sync
// changes take effect at runtime without a restart.
//
// It is a no-op when the KMS does not support rotation (single-key-file mode):
// such a KMS never fires rotation hooks anyway.
func applyAuthorityHintJWKSSyncHooks(
	entityID string,
	signer jwx.VersatileSigner,
	kvStorage model.KeyValueStore,
	hintStore model.AuthorityHintsStore,
	km adminapi.KeyManagement,
) error {
	if km.Keys == nil {
		return nil
	}
	rotationConf, err := storage.GetKeyRotation(kvStorage)
	if err != nil {
		return errors.Wrap(err, "could not read key rotation config for jwks sync hooks")
	}
	hints, err := hintStore.List()
	if err != nil {
		return errors.Wrap(err, "could not list authority hints for jwks sync hooks")
	}
	hooks, err := buildAuthorityHintJWKSSyncHooks(entityID, signer, hints)
	if err != nil {
		return err
	}
	rotationConf.Hooks = hooks
	// ChangeKeyRotationConfig always replaces the KMS's config (including
	// Hooks); it only skips (re)starting the rotation loop when the effective
	// rotation values are unchanged, so this also works on a running rotation.
	if err := km.Keys.ChangeKeyRotationConfig(rotationConf); err != nil {
		return errors.Wrap(err, "could not attach jwks sync hooks to key rotation")
	}
	return nil
}

// startAutomaticKeyRotation starts the KMS's automatic key rotation loop when
// enabled. It is a no-op when the KMS does not support rotation (single-key-file
// mode) or when the loop is already running. Called by NewLightHouse after
// JWKS-sync hooks have been attached (initKey loads the KMS without starting
// rotation).
func startAutomaticKeyRotation(kvStorage model.KeyValueStore, km adminapi.KeyManagement) error {
	if km.Keys == nil {
		return nil
	}
	rotationConf, err := storage.GetKeyRotation(kvStorage)
	if err != nil {
		return errors.Wrap(err, "could not read key rotation config")
	}
	if !rotationConf.Enabled {
		return nil
	}
	return km.Keys.StartAutomaticRotation()
}

// buildAuthorityHintJWKSSyncHooks builds the key rotation hooks that sync
// rotated federation keys to the configured authority hints. Each hint with a
// non-none jwks_sync.mode produces one hook. The returned slice is empty if no
// hint opts into syncing. Construction errors indicate misconfiguration (e.g.
// a missing signer) and are returned so they surface at startup.
func buildAuthorityHintJWKSSyncHooks(
	entityID string, signer jwx.VersatileSigner, hints []model.AuthorityHint,
) ([]kms.KeyRotationHook, error) {
	var hooks []kms.KeyRotationHook
	// The ROProducer is only needed for trigger mode; build it lazily.
	var roProducer *oidfed.RequestObjectProducer
	for _, hint := range hints {
		if hint.JWKSSync == nil {
			continue
		}
		switch hint.JWKSSync.Mode {
		case "", model.JWKSSyncNone:
			continue
		case model.JWKSSyncPush:
			hook, err := oidfed.JWKSUpdateHook(oidfed.JWKSUpdateHookConfig{
				TargetEntityID: hint.EntityID,
				Signer:         signer,
				JWTLifetime:    secondsDuration(hint.JWKSSync.JWTLifetime),
				Headers:        hint.JWKSSync.Headers,
				Timeout:        secondsDuration(hint.JWKSSync.Timeout),
			})
			if err != nil {
				return nil, errors.Wrapf(
					err, "could not build jwks sync (push) hook for authority hint %q",
					hint.EntityID,
				)
			}
			hooks = append(hooks, hook)
		case model.JWKSSyncTrigger:
			if roProducer == nil {
				roProducer = oidfed.NewRequestObjectProducer(entityID, signer, triggerAssertionLifetime)
			}
			hook, err := oidfed.TriggerUpdateHook(oidfed.TriggerUpdateHookConfig{
				TargetEntityID: hint.EntityID,
				ROProducer:     roProducer,
				Headers:        hint.JWKSSync.Headers,
				Timeout:        secondsDuration(hint.JWKSSync.Timeout),
			})
			if err != nil {
				return nil, errors.Wrapf(
					err, "could not build jwks sync (trigger) hook for authority hint %q",
					hint.EntityID,
				)
			}
			hooks = append(hooks, hook)
		}
	}
	return hooks, nil
}

// secondsDuration converts an int64 count of seconds to a time.Duration. A
// zero value yields zero, letting the lib apply its default (20s timeout, 10m
// JWT lifetime).
func secondsDuration(seconds int64) time.Duration {
	return time.Duration(seconds) * time.Second
}
