package config

import (
	"time"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse"
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

type SigningConf lighthouse.SigningConf

var defaultSigningConf = SigningConf{
	KMS:       lighthouse.KMSFilesystem,
	PKBackend: lighthouse.PKBackendDatabase,
	Alg:       "ES512",
	RSAKeyLen: 2048,
	KeyRotation: kms.KeyRotationConfig{
		Enabled:  false,
		Interval: duration.DurationOption(time.Second * 600000), // a little bit under a week
		Overlap:  duration.DurationOption(time.Hour),
	},
	AutoGenerateKeys: true,
}

func (c *SigningConf) validate() error {
	var ok bool
	c.Algorithm, ok = jwa.LookupSignatureAlgorithm(c.Alg)
	if !ok {
		return errors.New("error in signing conf: unknown algorithm " + c.Alg)
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
