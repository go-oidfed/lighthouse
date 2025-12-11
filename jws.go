package lighthouse

import (
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/api/adminapi"
	"github.com/go-oidfed/lighthouse/storage/model"
)

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

// TODO KMS to use config from DB

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
	switch c.KMS {
	case KMSFilesystem:
		if c.FileSystemBackend.KeyFile != "" {
			keyManagement.BasicKeys = &kms.SingleSigningKeyFile{
				Alg:  c.Algorithm,
				Path: c.FileSystemBackend.KeyFile,
			}
		} else {
			keyManagement.Keys = kms.NewSingleAlgFilesystemKMS(
				c.Algorithm, kms.FilesystemKMSConfig{
					KMSConfig: kms.KMSConfig{
						GenerateKeys: c.AutoGenerateKeys,
						RSAKeyLen:    c.RSAKeyLen,
						KeyRotation:  c.KeyRotation,
					},
					Dir:    c.FileSystemBackend.KeyDir,
					TypeID: "federation",
				}, keyManagement.KMSManagedPKs,
			)
		}
	case KMSPKCS11:
		keyManagement.Keys = kms.NewSingleAlgPKCS11KMS(
			c.Algorithm, kms.PKCS11KMSConfig{
				KMSConfig: kms.KMSConfig{
					GenerateKeys: c.AutoGenerateKeys,
					RSAKeyLen:    c.RSAKeyLen,
					KeyRotation:  c.KeyRotation,
				},
				TypeID:      "federation",
				ModulePath:  c.PKCS11Backend.ModulePath,
				TokenLabel:  c.PKCS11Backend.TokenLabel,
				TokenSerial: c.PKCS11Backend.TokenSerial,
				Pin:         c.PKCS11Backend.Pin,
				LabelPrefix: c.PKCS11Backend.LabelPrefix,
				ExtraLabels: c.PKCS11Backend.ExtraLabels,
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
	if keyManagement.Keys != nil && c.KeyRotation.Enabled {
		err = errors.Wrap(keyManagement.Keys.StartAutomaticRotation(), "could not start automatic key rotation")
		return
	}
	return
}
