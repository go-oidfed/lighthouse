package config

import (
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse"
)

// SigningConf holds signing configuration.
// Note: alg, rsa_key_len, and key_rotation are now managed in the database.
// Use 'lhmigrate config2db' to migrate these values from a config file,
// or use the Admin API to manage them at runtime.
type SigningConf struct {
	lighthouse.SigningConf `yaml:",inline"`
}

var defaultSigningConf = SigningConf{
	SigningConf: lighthouse.SigningConf{
		KMS:              lighthouse.KMSFilesystem,
		PKBackend:        lighthouse.PKBackendDatabase,
		AutoGenerateKeys: true,
	},
}

func (c *SigningConf) validate() error {
	switch c.KMS {
	case lighthouse.KMSFilesystem:
		if c.FileSystemBackend.KeyDir == "" && c.FileSystemBackend.KeyFile == "" {
			return errors.New("error in signing conf: filesystem.key_dir or filesystem.key_file must be specified")
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
