package config

import (
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse"
)

// SigningConf holds signing configuration.
// Note: alg, rsa_key_len, and key_rotation are now managed in the database.
// Use 'lhmigrate config2db' to migrate these values from a config file,
// or use the Admin API to manage them at runtime.
//
// Environment variables (with prefix LH_SIGNING_):
//   - LH_SIGNING_KMS: Key management system ("filesystem" or "pkcs11")
//   - LH_SIGNING_PK_BACKEND: Public key storage backend ("filesystem" or "db")
//   - LH_SIGNING_AUTO_GENERATE_KEYS: Auto-generate keys if missing
//   - LH_SIGNING_FILESYSTEM_KEY_FILE: Path to single key file
//   - LH_SIGNING_FILESYSTEM_KEY_DIR: Directory for key files
//   - LH_SIGNING_PKCS11_STORAGE_DIR: PKCS#11 storage directory
//   - LH_SIGNING_PKCS11_MODULE_PATH: Path to PKCS#11 module
//   - LH_SIGNING_PKCS11_TOKEN_LABEL: HSM token label
//   - LH_SIGNING_PKCS11_TOKEN_SERIAL: HSM token serial
//   - LH_SIGNING_PKCS11_TOKEN_SLOT: HSM slot number
//   - LH_SIGNING_PKCS11_PIN: HSM user PIN
//   - LH_SIGNING_PKCS11_MAX_SESSIONS: Maximum concurrent sessions
//   - LH_SIGNING_PKCS11_USER_TYPE: User type for login
//   - LH_SIGNING_PKCS11_NO_LOGIN: Token doesn't support login
//   - LH_SIGNING_PKCS11_LABEL_PREFIX: Prefix for object labels
//   - LH_SIGNING_PKCS11_LOAD_LABELS: Extra labels to load (comma-separated)
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
