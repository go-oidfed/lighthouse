package config

import (
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type legacySigningConf struct {
	Alg                  string                 `yaml:"alg"`
	Algorithm            jwa.SignatureAlgorithm `yaml:"-"`
	RSAKeyLen            int                    `yaml:"rsa_key_len"`
	KeyFile              string                 `yaml:"key_file"`
	KeyDir               string                 `yaml:"key_dir"`
	AutomaticKeyRollover struct {
		Enabled                   bool                    `yaml:"enabled"`
		Interval                  duration.DurationOption `yaml:"interval"`
		NumberOfOldKeysKeptInJWKS int                     `yaml:"old_keys_kept_in_jwks"`
		KeepHistory               bool                    `yaml:"keep_history"`
	} `yaml:"automatic_key_rollover"`
	AutoGenerateKeys bool `yaml:"auto_generate_keys"`
}

type SigningConf struct {
	lighthouse.SigningConf `yaml:",inline"`
	Alg                    string                 `yaml:"alg"`
	AlgOverwrite           bool                   `yaml:"alg#overwrite"`
	Algorithm              jwa.SignatureAlgorithm `yaml:"-"`
	RSAKeyLen              int                    `yaml:"rsa_key_len"`
	RSAKeyLenOverwrite     bool                   `yaml:"rsa_key_len#overwrite"`
	KeyRotation            struct {
		kms.KeyRotationConfig `yaml:",inline"`
		EnabledOverwrite      bool `yaml:"enabled#overwrite"`
		IntervalOverwrite     bool `yaml:"interval#overwrite"`
		OverlapOverwrite      bool `yaml:"overlap#overwrite"`
	} `yaml:"key_rotation"`
	KeyRotationOverwrite bool `yaml:"key_rotation#overwrite"`
}

func (c SigningConf) OverwriteDBValues(kvStore model.KeyValueStore) error {
	if c.Alg != "" {
		if c.AlgOverwrite {
			if err := errors.Wrap(
				storage.SetSigningAlg(kvStore, c.Algorithm), "failed to overwrite db signing alg",
			); err != nil {
				return err
			}
		} else {
			// Initialize only if unset in DB using raw GetAs
			var algStr string
			found, err := kvStore.GetAs(model.KeyValueScopeSigning, model.KeyValueKeyAlg, &algStr)
			if err != nil {
				return err
			}
			if !found {
				if err = storage.SetSigningAlg(kvStore, c.Algorithm); err != nil {
					return errors.Wrap(err, "failed to initialize db signing alg")
				}
			}
		}
	}
	if c.RSAKeyLen != 0 {
		if c.RSAKeyLenOverwrite {
			if err := errors.Wrap(
				storage.SetRSAKeyLen(kvStore, c.RSAKeyLen), "failed to overwrite db rsa key len",
			); err != nil {
				return err
			}
		} else {
			// Initialize only if unset in DB using raw GetAs
			var rsaLen int
			found, err := kvStore.GetAs(model.KeyValueScopeSigning, model.KeyValueKeyRSAKeyLen, &rsaLen)
			if err != nil {
				return err
			}
			if !found {
				if err = storage.SetRSAKeyLen(kvStore, c.RSAKeyLen); err != nil {
					return errors.Wrap(err, "failed to initialize db rsa key len")
				}
			}
		}
	}
	if err := c.overwriteDBRotation(kvStore); err != nil {
		return err
	}
	return nil
}

func (c SigningConf) overwriteDBRotation(kvStore model.KeyValueStore) error {
	if !c.KeyRotationOverwrite && !c.KeyRotation.EnabledOverwrite && !c.KeyRotation.IntervalOverwrite && !c.KeyRotation.OverlapOverwrite {
		var existing kms.KeyRotationConfig
		found, err := kvStore.GetAs(model.KeyValueScopeSigning, model.KeyValueKeyKeyRotation, &existing)
		if err != nil {
			return err
		}
		if !found {
			return errors.Wrap(
				storage.SetKeyRotation(kvStore, c.KeyRotation.KeyRotationConfig), "failed to initialize db key rotation",
			)
		}
		return nil
	}
	overwriteData := c.KeyRotation.KeyRotationConfig
	if !c.KeyRotationOverwrite {
		// Only do a partial overwrite
		var err error
		overwriteData, err = storage.GetKeyRotation(kvStore)
		if err != nil {
			return err
		}
		if c.KeyRotation.EnabledOverwrite {
			overwriteData.Enabled = c.KeyRotation.Enabled
		}
		if c.KeyRotation.IntervalOverwrite {
			overwriteData.Interval = c.KeyRotation.Interval
		}
		if c.KeyRotation.OverlapOverwrite {
			overwriteData.Overlap = c.KeyRotation.Overlap
		}
	}
	return errors.Wrap(storage.SetKeyRotation(kvStore, overwriteData), "failed to overwrite db key rotation")
}

var defaultSigningConf = SigningConf{
	SigningConf: lighthouse.SigningConf{
		KMS:              lighthouse.KMSFilesystem,
		PKBackend:        lighthouse.PKBackendDatabase,
		AutoGenerateKeys: true,
	},
	// The default values for alg, rsa_key_len and key_rotation are set in the db helper
}

func (c *SigningConf) validate() error {
	var ok bool
	if c.Alg != "" {
		c.Algorithm, ok = jwa.LookupSignatureAlgorithm(c.Alg)
		if !ok {
			return errors.New("error in signing conf: unknown algorithm " + c.Alg)
		}
	}
	switch c.KMS {
	case lighthouse.KMSFilesystem:
		if c.FileSystemBackend.KeyDir == "" && c.FileSystemBackend.KeyFile == "" {
			return errors.New("error in signing conf: filesystem.key_dir or filesystem.key_file must be specified")
		}
		if c.FileSystemBackend.KeyFile != "" {
			c.KeyRotation.Enabled = false
		}
	case lighthouse.KMSPKCS11:
		if c.PKCS11Backend.ModulePath == "" {
			return errors.New("error in signing conf: pkcs11.module_path must be specified")
		}
		if c.PKCS11Backend.TokenLabel == "" && c.PKCS11Backend.TokenSerial == "" && c.PKCS11Backend.SlotNumber == nil {
			return errors.New("error in signing conf: pkcs11.token_label, pkcs11.token_serial or pkcs11.slot_number must be specified")
		}
		if c.PKCS11Backend.Pin == "" && !c.PKCS11Backend.LoginNotSupported {
			return errors.New("error in signing conf: pkcs11.pin must be specified")
		}
	default:
		return errors.Errorf("error in signing conf: unknown KMS %s", c.KMS)
	}
	if c.PKBackend == lighthouse.PKBackendFilesystem && c.FileSystemBackend.KeyDir == "" {
		return errors.New("error in signing conf: filesystem.key_dir must be specified")
	}
	return nil
}
