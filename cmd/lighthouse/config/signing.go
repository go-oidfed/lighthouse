package config

import (
	"time"

	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/duration"
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
	KMS               string                 `yaml:"kms"`
	PKBackend         string                 `yaml:"pk_backend"`
	Alg               string                 `yaml:"alg"`
	Algorithm         jwa.SignatureAlgorithm `yaml:"-"`
	RSAKeyLen         int                    `yaml:"rsa_key_len"`
	KeyRotation       kms.KeyRotationConfig  `yaml:"key_rotation"`
	AutoGenerateKeys  bool                   `yaml:"auto_generate_keys"`
	FileSystemBackend struct {
		KeyFile string `yaml:"key_file"`
		KeyDir  string `yaml:"key_dir"`
	} `yaml:"filesystem"`
	PKCS11Backend struct {

		// ModulePath is the path to the PKCS#11 module (crypto11.Config.Path)
		ModulePath string `yaml:"module_path"`
		// TokenLabel selects the token by label (crypto11.Config.TokenLabel)
		TokenLabel string `yaml:"token_label"`
		// TokenSerial selects the token by serial (crypto11.Config.TokenSerial)
		TokenSerial string `yaml:"token_serial"`
		// SlotNumber selects the token by slot number (crypto11.Config.SlotNumber)
		SlotNumber *int `yaml:"token_slot"`
		// Pin is the user PIN for the token (crypto11.Config.Pin)
		Pin string `yaml:"pin"`

		// Maximum number of concurrent sessions to open. If zero, DefaultMaxSessions is used.
		// Otherwise, the value specified must be at least 2.
		MaxSessions int `yaml:"max_sessions"`

		// User type identifies the user type logging in. If zero, DefaultUserType is used.
		UserType int `yaml:"user_type"`

		// LoginNotSupported should be set to true for tokens that do not support logging in.
		LoginNotSupported bool `yaml:"no_login"`

		// Optional prefix for object labels inside HSM
		LabelPrefix string `yaml:"label_prefix"`

		// ExtraLabels are HSM object labels to load into this KMS even if
		// they are not present yet in the PublicKeyStorage.
		ExtraLabels []string `yaml:"load_labels"`
	} `yaml:"pkcs11"`
}

const (
	KMSFilesystem = "filesystem"
	KMSPKCS11     = "pkcs11"
)

const (
	PKBackendFilesystem = "filesystem"
	PKBackendDatabase   = "db"
)

var defaultSigningConf = SigningConf{
	KMS:       KMSFilesystem,
	PKBackend: PKBackendDatabase,
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
	case KMSFilesystem:
		if c.FileSystemBackend.KeyDir == "" && c.FileSystemBackend.KeyFile == "" {
			return errors.New("error in signing conf: filesystem.key_dir or filesystem.key_file must be specified")
		}
		if c.FileSystemBackend.KeyFile != "" {
			c.KeyRotation.Enabled = false
		}
	case KMSPKCS11:
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
	if c.PKBackend == PKBackendFilesystem && c.FileSystemBackend.KeyDir == "" {
		return errors.New("error in signing conf: filesystem.key_dir must be specified")
	}
	return nil
}
