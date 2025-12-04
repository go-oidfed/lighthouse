package main

import (
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/pkg/errors"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
)

var apiManagedPKs public.PublicKeyStorage
var kmsManagedPKs public.PublicKeyStorage
var basicKeys kms.BasicKeyManagementSystem
var keys kms.KeyManagementSystem

func initKey(c config.SigningConf, dbStorageFnc func(string) public.PublicKeyStorage) (
	err error,
) {
	switch c.PKBackend {
	case config.PKBackendFilesystem:
		kmsManagedPKs = &public.FilesystemPublicKeyStorage{
			Dir:    c.FileSystemBackend.KeyDir,
			TypeID: "federation",
		}
		apiManagedPKs = &public.FilesystemPublicKeyStorage{
			Dir:    c.FileSystemBackend.KeyDir,
			TypeID: "api",
		}
	case config.PKBackendDatabase:
		kmsManagedPKs = dbStorageFnc("federation")
		apiManagedPKs = dbStorageFnc("api")
	default:
		return errors.Errorf("signing.pk_backend %s not supported", c.PKBackend)
	}
	if err = kmsManagedPKs.Load(); err != nil {
		return
	}
	if err = apiManagedPKs.Load(); err != nil {
		return
	}
	switch c.KMS {
	case config.KMSFilesystem:
		if c.FileSystemBackend.KeyFile != "" {
			basicKeys = &kms.SingleSigningKeyFile{
				Alg:  c.Algorithm,
				Path: c.FileSystemBackend.KeyFile,
			}
		} else {
			keys = kms.NewSingleAlgFilesystemKMS(
				c.Algorithm, kms.FilesystemKMSConfig{
					KMSConfig: kms.KMSConfig{
						GenerateKeys: c.AutoGenerateKeys,
						RSAKeyLen:    c.RSAKeyLen,
						KeyRotation:  c.KeyRotation,
					},
					Dir:    c.FileSystemBackend.KeyDir,
					TypeID: "federation",
				}, kmsManagedPKs,
			)
		}
	case config.KMSPKCS11:
		keys = kms.NewSingleAlgPKCS11KMS(
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
			}, kmsManagedPKs,
		)
	default:
		return errors.Errorf("signing.kms %s not supported", c.KMS)
	}
	if keys != nil {
		basicKeys = keys
	}
	if err = errors.Wrap(basicKeys.Load(), "could not load kms"); err != nil {
		return
	}
	if keys != nil && c.KeyRotation.Enabled {
		return errors.Wrap(keys.StartAutomaticRotation(), "could not start automatic key rotation")
	}
	return
}

func versatileSigner() jwx.VersatileSigner {
	return kms.KMSToVersatileSignerWithJWKSFunc(
		basicKeys, func() (jwx.JWKS, error) {
			kmsHistory, err := kmsManagedPKs.GetValid()
			if err != nil {
				return jwx.JWKS{}, err
			}
			apiHistory, err := apiManagedPKs.GetValid()
			if err != nil {
				return jwx.JWKS{}, err
			}
			allEntries := append(kmsHistory, apiHistory...)
			set := jwx.NewJWKS()
			for _, k := range allEntries {
				kk, err := k.JWK()
				if err != nil {
					return jwx.JWKS{}, err
				}
				_ = set.AddKey(kk)
			}
			return set, nil
		},
	)
}
