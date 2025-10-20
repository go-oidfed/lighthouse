package main

import (
	"path/filepath"

	"github.com/go-oidfed/lib/jwx"
	"github.com/pkg/errors"
	"github.com/zachmann/go-utils/fileutils"

	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
)

var keys *jwx.KeyStorage

func initKey() (err error) {
	c := config.Get().Signing

	// If auto-generation is disabled, ensure the expected key file exists
	if !c.AutoGenerateKeys {
		keyFilename := "federation_" + c.Alg + ".pem"
		keyPath := filepath.Join(c.KeyDir, keyFilename)
		if !fileutils.FileExists(keyPath) {
			return errors.Errorf(
				"signing.auto_generate_keys is false, "+
					"but required key not found: %s. Provide an existing key or enable auto generation.", keyPath,
			)
		}
	}

	keys, err = jwx.NewKeyStorage(
		c.KeyDir, map[string]jwx.KeyStorageConfig{
			jwx.KeyStorageTypeFederation: {
				Algorithm:    c.Alg,
				RSAKeyLen:    c.RSAKeyLen,
				RolloverConf: c.AutomaticKeyRollover,
			},
		},
	)
	if err != nil {
		return
	}
	return keys.Load()
}
