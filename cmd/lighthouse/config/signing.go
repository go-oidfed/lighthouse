package config

import (
	"github.com/go-oidfed/lib/jwx"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
)

type signingConf struct {
	Alg                  string                 `yaml:"alg"`
	Algorithm            jwa.SignatureAlgorithm `yaml:"-"`
	RSAKeyLen            int                    `yaml:"rsa_key_len"`
	KeyFile              string                 `yaml:"key_file"`
	KeyDir               string                 `yaml:"key_dir"`
	AutomaticKeyRollover jwx.RolloverConf       `yaml:"automatic_key_rollover"`
	AutoGenerateKeys     bool                   `yaml:"auto_generate_keys"`
}

var defaultSigningConf = signingConf{
	Alg:              "ES512",
	RSAKeyLen:        2048,
	AutoGenerateKeys: true,
	AutomaticKeyRollover: jwx.RolloverConf{
		Enabled:  false,
		Interval: 600000,
	},
}

func (c *signingConf) validate() error {
	if c.KeyFile != "" {
		return errors.New(
			"'signing.key_file' is deprecated, " +
				"use 'signing.key_dir' instead\nTo keep the existing signing key" +
				" place it in the 'signing.key_dir' directory (" +
				"if not already the case) and rename it to the following naming" +
				" scheme:\nfederation_<alg>.pem\nExample: federation_ES512.pem",
		)
	}
	if c.KeyDir == "" {
		return errors.New("error in signing conf: key_dir must be specified")
	}
	var ok bool
	c.Algorithm, ok = jwa.LookupSignatureAlgorithm(c.Alg)
	if !ok {
		return errors.New("error in signing conf: unknown algorithm " + c.Alg)
	}
	return nil
}
