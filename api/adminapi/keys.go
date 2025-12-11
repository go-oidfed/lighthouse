package adminapi

import (
	"errors"
	"slices"
	"time"

	oidfed "github.com/go-oidfed/lib"
	"github.com/go-oidfed/lib/jwx"
	"github.com/go-oidfed/lib/jwx/keymanagement/kms"
	"github.com/go-oidfed/lib/jwx/keymanagement/public"
	"github.com/go-oidfed/lib/unixtime"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/zachmann/go-utils/duration"

	"github.com/go-oidfed/lighthouse/storage"
	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

type KeyManagement struct {
	KMS           string
	APIManagedPKs public.PublicKeyStorage
	KMSManagedPKs public.PublicKeyStorage
	BasicKeys     kms.BasicKeyManagementSystem
	Keys          kms.KeyManagementSystem
}

type kmsInfo struct {
	KMS       string                `json:"kms"`
	Alg       string                `json:"alg"`
	RSAKeyLen int                   `json:"rsa_key_len"`
	Rotation  kms.KeyRotationConfig `json:"rotation"`
}

func addKeysToSet(set jwx.JWKS, keys public.PublicKeyEntryList) error {
	for _, pub := range keys {
		k, err := pub.JWK()
		if err != nil {
			return err
		}
		_ = set.AddKey(k)
	}
	return nil
}

// registerKeys wires routes for managing public keys and KMS-related endpoints.
func registerKeys(r fiber.Router, keyManagement KeyManagement, kvStorage smodel.KeyValueStore) {
	// Published JWKS (as seen in entity configuration)
	r.Get(
		"/entity-configuration/jwks", func(c *fiber.Ctx) error {
			set := jwx.NewJWKS()
			addValidKeys := func(pkStorage public.PublicKeyStorage) error {
				list, err := pkStorage.GetValid()
				if err != nil {
					return err
				}
				return addKeysToSet(set, list)
			}
			if err := addValidKeys(keyManagement.KMSManagedPKs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err := addValidKeys(keyManagement.APIManagedPKs); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(set)
		},
	)

	// Public keys collection
	g := r.Group("/entity-configuration/keys")

	apiManagedPKs := keyManagement.APIManagedPKs

	// GET: list API-managed public keys
	g.Get(
		"/", func(c *fiber.Ctx) error {
			keys, err := apiManagedPKs.GetAll()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(keys)
		},
	)

	// POST: add a new public key
	g.Post(
		"/", func(c *fiber.Ctx) error {
			var req public.PublicKeyEntry
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.Key.Key == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("missing key"))
			}
			// Ensure kid
			kid, ok := req.Key.KeyID()
			if !ok {
				_ = jwk.AssignKeyID(req.Key.Key)
				kid, _ = req.Key.KeyID()
			}
			if req.KID == "" {
				req.KID = kid
			} else if req.KID != kid {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("provided 'kid' does not match key"))
			}
			err := apiManagedPKs.Add(req)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Fetch the created entry to return it
			created, err := apiManagedPKs.Get(kid)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(created)
		},
	)

	// PUT: update key metadata (exp only)
	type updateReq struct {
		Exp *unixtime.Unixtime `json:"exp"`
	}
	g.Put(
		"/:kid", func(c *fiber.Ctx) error {
			kid := c.Params("kid")
			// Use API-managed storage
			var req updateReq
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			err := apiManagedPKs.Update(
				kid, public.UpdateablePublicKeyMetadata{
					ExpiresAt: req.Exp,
				},
			)
			if err != nil {
				var nf public.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(nf.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Fetch the updated entry to return it
			updated, err := apiManagedPKs.Get(kid)
			if err != nil {
				var nf public.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(nf.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if updated == nil {
				return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound("key not found"))
			}
			return c.JSON(updated)
		},
	)

	// POST kid: rotate a public key
	type rotateReq struct {
		Key       public.JWKKey      `json:"key"`
		Iat       *unixtime.Unixtime `json:"iat"`
		Nbf       *unixtime.Unixtime `json:"nbf"`
		Exp       *unixtime.Unixtime `json:"exp"`
		OldKeyExp *unixtime.Unixtime `json:"old_key_exp"`
	}
	g.Post(
		"/:kid", func(c *fiber.Ctx) error {
			oldKid := c.Params("kid")
			// Use API-managed storage
			var req rotateReq
			if err := c.BodyParser(&req); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if req.Key.Key == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("missing key"))
			}
			// Ensure kid
			kid, ok := req.Key.KeyID()
			if !ok {
				_ = jwk.AssignKeyID(req.Key.Key)
				kid, _ = req.Key.KeyID()
			}
			if req.Nbf == nil {
				now := unixtime.Now()
				req.Nbf = &now
			}
			rotationConf, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			oldKeyExpiration := req.OldKeyExp
			if oldKeyExpiration == nil {
				oldKeyExpiration = &unixtime.Unixtime{Time: req.Nbf.Add(rotationConf.Overlap.Duration())}
			}
			err = apiManagedPKs.Update(
				oldKid, public.UpdateablePublicKeyMetadata{
					ExpiresAt: oldKeyExpiration,
				},
			)
			if err != nil {
				var nf public.NotFoundError
				if errors.As(err, &nf) {
					return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(nf.Error()))
				}
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err = apiManagedPKs.Add(
				public.PublicKeyEntry{
					KID:       kid,
					Key:       req.Key,
					IssuedAt:  req.Iat,
					NotBefore: req.Nbf,
					UpdateablePublicKeyMetadata: public.UpdateablePublicKeyMetadata{
						ExpiresAt: req.Exp,
					},
				},
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			// Fetch the created entry to return it
			created, err := apiManagedPKs.Get(kid)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.Status(fiber.StatusCreated).JSON(created)
		},
	)

	// DELETE kid: remove or revoke
	g.Delete(
		"/:kid", func(c *fiber.Ctx) error {
			kid := c.Params("kid")
			revoke := c.QueryBool("revoke", false)
			reason := c.Query("reason")
			// Use API-managed storage
			if revoke {
				_ = apiManagedPKs.Revoke(kid, reason)
				return c.SendStatus(fiber.StatusNoContent)
			}
			_ = apiManagedPKs.Delete(kid)
			return c.SendStatus(fiber.StatusNoContent)
		},
	)

	// KMS info
	r.Get(
		"/kms", func(c *fiber.Ctx) error {
			alg, err := storage.GetSigningAlg(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			rotation, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			rsaKeyLen, err := storage.GetRSAKeyLen(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			return c.JSON(
				kmsInfo{
					KMS:       keyManagement.KMS,
					Alg:       alg.String(),
					RSAKeyLen: rsaKeyLen,
					Rotation:  rotation,
				},
			)
		},
	)

	r.Put(
		"/kms/alg", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support changing signing alg dynamically"))
			}
			var alg string
			if err := c.BodyParser(&alg); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			jwaAlg, ok := jwa.LookupSignatureAlgorithm(alg)
			if !ok {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid algorithm"))
			}
			if !slices.Contains(jwx.SupportedAlgsStrings(), alg) {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("unsupported algorithm"))
			}
			if err := storage.SetSigningAlg(kvStorage, jwaAlg); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			rotation, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			rsaKeyLen, err := storage.GetRSAKeyLen(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err = keyManagement.Keys.ChangeAlgs([]jwa.SignatureAlgorithm{jwaAlg}); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err = keyManagement.Keys.ChangeDefaultAlgorithm(jwaAlg); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			return c.JSON(
				kmsInfo{
					KMS:       keyManagement.KMS,
					Alg:       alg,
					RSAKeyLen: rsaKeyLen,
					Rotation:  rotation,
				},
			)

		},
	)

	r.Put(
		"/kms/rsa-key-len", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support changing RSA key length dynamically"))
			}
			var rsaKeyLen int
			if err := c.BodyParser(&rsaKeyLen); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := storage.SetRSAKeyLen(kvStorage, rsaKeyLen); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			rotation, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			alg, err := storage.GetSigningAlg(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err = keyManagement.Keys.ChangeRSAKeyLength(rsaKeyLen); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}

			return c.JSON(
				kmsInfo{
					KMS:       keyManagement.KMS,
					Alg:       alg.String(),
					RSAKeyLen: rsaKeyLen,
					Rotation:  rotation,
				},
			)

		},
	)

	// KMS rotation options (stored in KV)
	r.Get(
		"/kms/rotation", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
			}
			rot, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(rot)
		},
	)
	r.Put(
		"/kms/rotation", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
			}
			var cfg kms.KeyRotationConfig
			if err := c.BodyParser(&cfg); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if err := storage.SetKeyRotation(kvStorage, cfg); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err := keyManagement.Keys.ChangeKeyRotationConfig(cfg); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(cfg)
		},
	)
	r.Patch(
		"/kms/rotation", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
			}
			// Load existing config
			current, err := storage.GetKeyRotation(kvStorage)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			// Apply partial updates
			var patch map[string]any
			if err = c.BodyParser(&patch); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
			}
			if v, ok := patch["enabled"].(bool); ok {
				current.Enabled = v
			}
			if v, ok := patch["interval"].(float64); ok {
				current.Interval = duration.DurationOption(time.Duration(v) * time.Second)
			}
			if v, ok := patch["overlap"].(float64); ok {
				current.Overlap = duration.DurationOption(time.Duration(v) * time.Second)
			}
			if err = storage.SetKeyRotation(kvStorage, current); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			if err = keyManagement.Keys.ChangeKeyRotationConfig(current); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.JSON(current)
		},
	)

	// Trigger KMS rotation (placeholder)
	r.Post(
		"/kms/rotate", func(c *fiber.Ctx) error {
			if keyManagement.Keys == nil {
				return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
			}
			revoke := c.QueryBool("revoke", false)
			reason := c.Query("reason")
			err := keyManagement.Keys.RotateAllKeys(revoke, reason)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
			}
			return c.SendStatus(fiber.StatusAccepted)
		},
	)
}
