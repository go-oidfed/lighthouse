package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwks"
)

type delegationConfig struct {
	TrustMarkOwner string                    `yaml:"trust_mark_owner" json:"trust_mark_owner"`
	JWKS           jwks.JWKS                 `yaml:"jwks" json:"jwks"`
	SigningKey     string                    `yaml:"signing_key" json:"signing_key"`
	TrustMarks     []delegationTrustMarkSpec `yaml:"trust_marks" json:"trust_marks"`
}

type delegationTrustMarkSpec struct {
	TrustMarkType      string            `yaml:"trust_mark_type" json:"trust_mark_type"`
	DelegationLifetime int64             `yaml:"delegation_lifetime" json:"delegation_lifetime"`
	Ref                string            `yaml:"ref" json:"ref"`
	TrustMarkIssuers   []delegatedEntity `yaml:"trust_mark_issuers" json:"trust_mark_issuers"`
}

type delegatedEntity struct {
	EntityID      string `yaml:"entity_id" json:"entity_id"`
	DelegationJWT string `yaml:"delegation_jwt" json:"delegation_jwt"`
}

var delegationJWTCmd = &cobra.Command{
	Use:   "delegation",
	Short: "Generate TM delegation JWTs",
	Long:  `Generate trust mark delegation JWTs`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDelegation,
}

var useJSONOutput bool

func init() {
	delegationJWTCmd.Flags().BoolVar(&useJSONOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(delegationJWTCmd)
}

func runDelegation(cmd *cobra.Command, args []string) error {
	confFile := args[0]
	content, err := os.ReadFile(confFile)
	if err != nil {
		return errors.Wrap(err, "failed to read configuration file")
	}
	var conf delegationConfig
	if err = yaml.Unmarshal(content, &conf); err != nil {
		return errors.Wrap(err, "failed to parse configuration file")
	}
	var sk *ecdsa.PrivateKey
	if conf.SigningKey == "" {
		sk, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return errors.Wrap(err, "failed to generate signing key")
		}
		privkeyBytes, _ := x509.MarshalECPrivateKey(sk)
		privkeyPem := pem.EncodeToMemory(
			&pem.Block{
				Type:  "EC PRIVATE KEY",
				Bytes: privkeyBytes,
			},
		)
		conf.SigningKey = string(privkeyPem)
	} else {
		block, _ := pem.Decode([]byte(conf.SigningKey))
		sk, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return errors.Wrap(err, "failed to parse signing key")
		}
	}
	if conf.JWKS.Set == nil {
		conf.JWKS = jwks.KeyToJWKS(sk.PublicKey, jwa.ES512())
	}

	ownedTrustMarks := make([]oidfed.OwnedTrustMark, len(conf.TrustMarks))
	for i, c := range conf.TrustMarks {
		ownedTrustMarks[i] = oidfed.OwnedTrustMark{
			ID:                 c.TrustMarkType,
			DelegationLifetime: time.Duration(c.DelegationLifetime) * time.Second,
			Ref:                c.Ref,
		}
	}

	tmo := oidfed.NewTrustMarkOwner(
		conf.TrustMarkOwner,
		oidfed.NewGeneralJWTSigner(sk, jwa.ES512()).TrustMarkDelegationSigner(),
		ownedTrustMarks,
	)
	for i, c := range conf.TrustMarks {
		for j, e := range c.TrustMarkIssuers {
			delegation, err := tmo.DelegationJWT(c.TrustMarkType, e.EntityID)
			if err != nil {
				return errors.Wrap(err, "failed to generate delegation JWT")
			}
			c.TrustMarkIssuers[j].DelegationJWT = string(delegation)
		}
		conf.TrustMarks[i] = c
	}

	var updatedConfig []byte
	if useJSONOutput {
		updatedConfig, err = json.Marshal(conf)
		confFile = strings.TrimSuffix(confFile, ".yaml")
		confFile = strings.TrimSuffix(confFile, ".yml")
		confFile += ".json"
	} else {
		updatedConfig, err = yaml.Marshal(conf)
	}
	if err != nil {
		return errors.Wrap(err, "failed to marshal configuration")
	}
	return os.WriteFile(confFile, updatedConfig, 0644)
}
