package lighthouse

import (
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// SigningConf holds signing configuration.
//
// Environment variables (with prefix LH_SIGNING_):
//   - LH_SIGNING_KMS: Key management system ("filesystem" or "pkcs11")
//   - LH_SIGNING_PK_BACKEND: Public key storage backend ("filesystem" or "db")
//   - LH_SIGNING_AUTO_GENERATE_KEYS: Auto-generate keys if missing (bool)
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
//   - LH_SIGNING_PKCS11_NO_LOGIN: Token doesn't support login (bool)
//   - LH_SIGNING_PKCS11_LABEL_PREFIX: Prefix for object labels
//   - LH_SIGNING_PKCS11_LOAD_LABELS: Extra labels to load (comma-separated)
type SigningConf struct {
	// KMS specifies the key management system to use.
	// Env: LH_SIGNING_KMS
	KMS string `yaml:"kms" envconfig:"KMS"`
	// PKBackend specifies the public key storage backend.
	// Env: LH_SIGNING_PK_BACKEND
	PKBackend string `yaml:"pk_backend" envconfig:"PK_BACKEND"`
	// AutoGenerateKeys enables automatic key generation if keys are missing.
	// Env: LH_SIGNING_AUTO_GENERATE_KEYS
	AutoGenerateKeys bool `yaml:"auto_generate_keys" envconfig:"AUTO_GENERATE_KEYS"`
	// FileSystemBackend holds filesystem-based key storage configuration.
	// Env prefix: LH_SIGNING_FILESYSTEM_
	FileSystemBackend struct {
		// KeyFile is the path to a single key file.
		// Env: LH_SIGNING_FILESYSTEM_KEY_FILE
		KeyFile string `yaml:"key_file" envconfig:"KEY_FILE"`
		// KeyDir is the directory for key files.
		// Env: LH_SIGNING_FILESYSTEM_KEY_DIR
		KeyDir string `yaml:"key_dir" envconfig:"KEY_DIR"`
	} `yaml:"filesystem" envconfig:"FILESYSTEM"`
	// PKCS11Backend holds PKCS#11 (HSM) configuration.
	// Env prefix: LH_SIGNING_PKCS11_
	PKCS11Backend struct {
		// StorageDir is the storage directory for PKCS#11.
		// Env: LH_SIGNING_PKCS11_STORAGE_DIR
		StorageDir string `yaml:"storage_dir" envconfig:"STORAGE_DIR"`

		// ModulePath is the path to the PKCS#11 module (crypto11.Config.Path)
		// Env: LH_SIGNING_PKCS11_MODULE_PATH
		ModulePath string `yaml:"module_path" envconfig:"MODULE_PATH"`
		// TokenLabel selects the token by label (crypto11.Config.TokenLabel)
		// Env: LH_SIGNING_PKCS11_TOKEN_LABEL
		TokenLabel string `yaml:"token_label" envconfig:"TOKEN_LABEL"`
		// TokenSerial selects the token by serial (crypto11.Config.TokenSerial)
		// Env: LH_SIGNING_PKCS11_TOKEN_SERIAL
		TokenSerial string `yaml:"token_serial" envconfig:"TOKEN_SERIAL"`
		// SlotNumber selects the token by slot number (crypto11.Config.SlotNumber)
		// Env: LH_SIGNING_PKCS11_TOKEN_SLOT
		SlotNumber *int `yaml:"token_slot" envconfig:"TOKEN_SLOT"`
		// Pin is the user PIN for the token (crypto11.Config.Pin)
		// Env: LH_SIGNING_PKCS11_PIN
		Pin string `yaml:"pin" envconfig:"PIN"`

		// MaxSessions is the maximum number of concurrent sessions to open.
		// If zero, DefaultMaxSessions is used. Otherwise, must be at least 2.
		// Env: LH_SIGNING_PKCS11_MAX_SESSIONS
		MaxSessions int `yaml:"max_sessions" envconfig:"MAX_SESSIONS"`

		// UserType identifies the user type logging in. If zero, DefaultUserType is used.
		// Env: LH_SIGNING_PKCS11_USER_TYPE
		UserType int `yaml:"user_type" envconfig:"USER_TYPE"`

		// LoginNotSupported should be set to true for tokens that do not support logging in.
		// Env: LH_SIGNING_PKCS11_NO_LOGIN
		LoginNotSupported bool `yaml:"no_login" envconfig:"NO_LOGIN"`

		// LabelPrefix is an optional prefix for object labels inside HSM.
		// Env: LH_SIGNING_PKCS11_LABEL_PREFIX
		LabelPrefix string `yaml:"label_prefix" envconfig:"LABEL_PREFIX"`

		// ExtraLabels are HSM object labels to load into this KMS even if
		// they are not present yet in the PublicKeyStorage.
		// Env: LH_SIGNING_PKCS11_LOAD_LABELS (comma-separated)
		ExtraLabels []string `yaml:"load_labels" envconfig:"LOAD_LABELS"`
	} `yaml:"pkcs11" envconfig:"PKCS11"`
}

const (
	KMSFilesystem = "filesystem"
	KMSPKCS11     = "pkcs11"
)

const (
	PKBackendFilesystem = "filesystem"
	PKBackendDatabase   = "db"
)

func initKey(c SigningConf, storages model.Backends) (
	keyManagement adminapi.KeyManagement,
	err error,
) {
	keyManagement.KMS = c.KMS
	switch c.PKBackend {
	case PKBackendFilesystem:
		keyManagement.KMSManagedPKs = &public.FilesystemPublicKeyStorage{
			Dir:    c.FileSystemBackend.KeyDir,
			TypeID: "federation",
		}
		keyManagement.APIManagedPKs = &public.FilesystemPublicKeyStorage{
			Dir:    c.FileSystemBackend.KeyDir,
			TypeID: "api",
		}
	case PKBackendDatabase:
		keyManagement.KMSManagedPKs = storages.PKStorages("federation")
		keyManagement.APIManagedPKs = storages.PKStorages("api")
	default:
		err = errors.Errorf("unsupported public key backend '%s'", c.PKBackend)
		return
	}
	if err = keyManagement.KMSManagedPKs.Load(); err != nil {
		return
	}
	if err = keyManagement.APIManagedPKs.Load(); err != nil {
		return
	}
	alg, e := storage.GetSigningAlg(storages.KV)
	if e != nil {
		err = e
		return
	}
	rsaKeyLen, e := storage.GetRSAKeyLen(storages.KV)
	if e != nil {
		err = e
		return
	}
	rotationConf, e := storage.GetKeyRotation(storages.KV)
	if e != nil {
		err = e
		return
	}
	switch c.KMS {
	case KMSFilesystem:
		if c.FileSystemBackend.KeyFile != "" {
			keyManagement.BasicKeys = &kms.SingleSigningKeyFile{
				Alg:  alg,
				Path: c.FileSystemBackend.KeyFile,
			}
		} else {
			keyManagement.Keys = kms.NewSingleAlgFilesystemKMS(
				alg, kms.FilesystemKMSConfig{
					KMSConfig: kms.KMSConfig{
						GenerateKeys: c.AutoGenerateKeys,
						RSAKeyLen:    rsaKeyLen,
						KeyRotation:  rotationConf,
					},
					Dir:    c.FileSystemBackend.KeyDir,
					TypeID: "federation",
				}, keyManagement.KMSManagedPKs,
			)
		}
	case KMSPKCS11:
		keyManagement.Keys = kms.NewSingleAlgPKCS11KMS(
			alg, kms.PKCS11KMSConfig{
				KMSConfig: kms.KMSConfig{
					GenerateKeys: c.AutoGenerateKeys,
					RSAKeyLen:    rsaKeyLen,
					KeyRotation:  rotationConf,
				},
				TypeID:            "federation",
				StorageDir:        c.PKCS11Backend.StorageDir,
				ModulePath:        c.PKCS11Backend.ModulePath,
				TokenLabel:        c.PKCS11Backend.TokenLabel,
				TokenSerial:       c.PKCS11Backend.TokenSerial,
				SlotNumber:        c.PKCS11Backend.SlotNumber,
				Pin:               c.PKCS11Backend.Pin,
				MaxSessions:       c.PKCS11Backend.MaxSessions,
				UserType:          c.PKCS11Backend.UserType,
				LoginNotSupported: c.PKCS11Backend.LoginNotSupported,
				LabelPrefix:       c.PKCS11Backend.LabelPrefix,
				ExtraLabels:       c.PKCS11Backend.ExtraLabels,
			}, keyManagement.KMSManagedPKs,
		)
	default:
		err = errors.Errorf("unsupported kms '%s'", c.PKBackend)
		return
	}
	if keyManagement.Keys != nil {
		keyManagement.BasicKeys = keyManagement.Keys
	}
	if err = errors.Wrap(keyManagement.BasicKeys.Load(), "could not load kms"); err != nil {
		return
	}
	if keyManagement.Keys != nil && rotationConf.Enabled {
		err = errors.Wrap(keyManagement.Keys.StartAutomaticRotation(), "could not start automatic key rotation")
		return
	}
	return
}
