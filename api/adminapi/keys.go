package adminapi

import (
	"errors"
	"slices"
	"strconv"
	"strings"
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

// KeyManagement holds references to key management components.
type KeyManagement struct {
	KMS           string
	APIManagedPKs public.PublicKeyStorage
	KMSManagedPKs public.PublicKeyStorage
	BasicKeys     kms.BasicKeyManagementSystem
	Keys          kms.KeyManagementSystem
}

type kmsInfo struct {
	KMS         string                `json:"kms"`
	Alg         string                `json:"alg"`
	PendingAlg  string                `json:"pending_alg,omitempty"`
	AlgChangeAt *unixtime.Unixtime    `json:"alg_change_at,omitempty"`
	RSAKeyLen   int                   `json:"rsa_key_len"`
	Rotation    kms.KeyRotationConfig `json:"rotation"`
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

// jwksHandlers groups handlers for JWKS endpoints.
type jwksHandlers struct {
	keyManagement KeyManagement
}

func (h *jwksHandlers) getJWKS(c *fiber.Ctx) error {
	set := jwx.NewJWKS()
	addValidKeys := func(pkStorage public.PublicKeyStorage) error {
		list, err := pkStorage.GetValid()
		if err != nil {
			return err
		}
		return addKeysToSet(set, list)
	}
	if err := addValidKeys(h.keyManagement.KMSManagedPKs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err := addValidKeys(h.keyManagement.APIManagedPKs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(set)
}

// publicKeyHandlers groups handlers for public key endpoints.
type publicKeyHandlers struct {
	apiManagedPKs public.PublicKeyStorage
	storages      smodel.Backends
	kvStorage     smodel.KeyValueStore
}

func (h *publicKeyHandlers) list(c *fiber.Ctx) error {
	keys, err := h.apiManagedPKs.GetAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(keys)
}

func (h *publicKeyHandlers) create(c *fiber.Ctx) error {
	var req public.PublicKeyEntry
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.Key.Key == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("missing key"))
	}
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
	if err := h.apiManagedPKs.Add(req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	created, err := h.apiManagedPKs.Get(kid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

type updateKeyReq struct {
	Exp *unixtime.Unixtime `json:"exp"`
}

func (h *publicKeyHandlers) update(c *fiber.Ctx) error {
	kid := c.Params("kid")
	var req updateKeyReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	err := h.apiManagedPKs.Update(kid, public.UpdateablePublicKeyMetadata{ExpiresAt: req.Exp})
	if err != nil {
		var nf public.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(nf.Error()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	updated, err := h.apiManagedPKs.Get(kid)
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
}

type rotateKeyReq struct {
	Key       public.JWKKey      `json:"key"`
	Iat       *unixtime.Unixtime `json:"iat"`
	Nbf       *unixtime.Unixtime `json:"nbf"`
	Exp       *unixtime.Unixtime `json:"exp"`
	OldKeyExp *unixtime.Unixtime `json:"old_key_exp"`
}

func (h *publicKeyHandlers) rotate(c *fiber.Ctx) error {
	oldKid := c.Params("kid")
	var req rotateKeyReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if req.Key.Key == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("missing key"))
	}
	kid, ok := req.Key.KeyID()
	if !ok {
		_ = jwk.AssignKeyID(req.Key.Key)
		kid, _ = req.Key.KeyID()
	}
	if req.Nbf == nil {
		now := unixtime.Now()
		req.Nbf = &now
	}
	rotationConf, err := storage.GetKeyRotation(h.kvStorage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	oldKeyExpiration := req.OldKeyExp
	if oldKeyExpiration == nil {
		oldKeyExpiration = &unixtime.Unixtime{Time: req.Nbf.Add(rotationConf.Overlap.Duration())}
	}

	var created *public.PublicKeyEntry
	err = h.storages.InTransaction(func(tx *smodel.Backends) error {
		txPKStorage := tx.PKStorages("api-managed")
		if err := txPKStorage.Update(oldKid, public.UpdateablePublicKeyMetadata{ExpiresAt: oldKeyExpiration}); err != nil {
			return err
		}
		if err := txPKStorage.Add(public.PublicKeyEntry{
			KID:                         kid,
			Key:                         req.Key,
			IssuedAt:                    req.Iat,
			NotBefore:                   req.Nbf,
			UpdateablePublicKeyMetadata: public.UpdateablePublicKeyMetadata{ExpiresAt: req.Exp},
		}); err != nil {
			return err
		}
		var err error
		created, err = txPKStorage.Get(kid)
		return err
	})

	if err != nil {
		var nf public.NotFoundError
		if errors.As(err, &nf) {
			return c.Status(fiber.StatusNotFound).JSON(oidfed.ErrorNotFound(nf.Error()))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *publicKeyHandlers) delete(c *fiber.Ctx) error {
	kid := c.Params("kid")
	revoke := c.QueryBool("revoke", false)
	reason := c.Query("reason")
	if revoke {
		_ = h.apiManagedPKs.Revoke(kid, reason)
		return c.SendStatus(fiber.StatusNoContent)
	}
	_ = h.apiManagedPKs.Delete(kid)
	return c.SendStatus(fiber.StatusNoContent)
}

// kmsHandlers groups handlers for KMS endpoints.
type kmsHandlers struct {
	keyManagement KeyManagement
	kvStorage     smodel.KeyValueStore
}

func (h *kmsHandlers) getInfo(c *fiber.Ctx) error {
	info, err := h.buildKMSInfo()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(info)
}

func (h *kmsHandlers) putAlg(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support changing signing alg dynamically"))
	}
	alg := strings.TrimSpace(string(c.Body()))
	if alg == "" {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("empty body"))
	}
	jwaAlg, ok := jwa.LookupSignatureAlgorithm(alg)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid algorithm"))
	}
	if !slices.Contains(jwx.SupportedAlgsStrings(), alg) {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("unsupported algorithm"))
	}

	ecLifetime, err := storage.GetEntityConfigurationLifetime(h.kvStorage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	rot, err := storage.GetKeyRotation(h.kvStorage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	switchTime := unixtime.Unixtime{Time: time.Now().Add(ecLifetime).Add(10 * time.Second)}
	if err = h.keyManagement.Keys.ChangeAlgsAt([]jwa.SignatureAlgorithm{jwaAlg}, switchTime, rot.Overlap.Duration()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err = h.keyManagement.Keys.ChangeDefaultAlgorithmAt(jwaAlg, switchTime); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err = storage.SetSigningAlg(h.kvStorage, storage.SigningAlgWithNbf{SigningAlg: alg, Nbf: &switchTime}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	info, err := h.buildKMSInfo()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(info)
}

func (h *kmsHandlers) putRSAKeyLen(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support changing RSA key length dynamically"))
	}
	rsaKeyLen, err := strconv.Atoi(strings.TrimSpace(string(c.Body())))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body: expected integer"))
	}
	if err := storage.SetRSAKeyLen(h.kvStorage, rsaKeyLen); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err := h.keyManagement.Keys.ChangeRSAKeyLength(rsaKeyLen); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	info, err := h.buildKMSInfo()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(info)
}

func (h *kmsHandlers) getRotation(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
	}
	rot, err := storage.GetKeyRotation(h.kvStorage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(rot)
}

func (h *kmsHandlers) putRotation(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
	}
	var cfg kms.KeyRotationConfig
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("invalid body"))
	}
	if err := storage.SetKeyRotation(h.kvStorage, cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err := h.keyManagement.Keys.ChangeKeyRotationConfig(cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(cfg)
}

func (h *kmsHandlers) patchRotation(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
	}
	current, err := storage.GetKeyRotation(h.kvStorage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
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
	if err = storage.SetKeyRotation(h.kvStorage, current); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	if err = h.keyManagement.Keys.ChangeKeyRotationConfig(current); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.JSON(current)
}

func (h *kmsHandlers) triggerRotate(c *fiber.Ctx) error {
	if h.keyManagement.Keys == nil {
		return c.Status(fiber.StatusBadRequest).JSON(oidfed.ErrorInvalidRequest("kms does not support rotation"))
	}
	revoke := c.QueryBool("revoke", false)
	reason := c.Query("reason")
	if err := h.keyManagement.Keys.RotateAllKeys(revoke, reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(oidfed.ErrorServerError(err.Error()))
	}
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *kmsHandlers) buildKMSInfo() (*kmsInfo, error) {
	alg := h.keyManagement.BasicKeys.GetDefaultAlg()
	rotation, err := storage.GetKeyRotation(h.kvStorage)
	if err != nil {
		return nil, err
	}
	rsaKeyLen, err := storage.GetRSAKeyLen(h.kvStorage)
	if err != nil {
		return nil, err
	}
	var pendingAlg string
	var pendingEffective *unixtime.Unixtime
	if h.keyManagement.Keys != nil {
		_, pending := h.keyManagement.Keys.GetPendingChanges()
		if pending != nil {
			pendingAlg = pending.Alg.String()
			pendingEffective = &pending.EffectiveAt
		}
	}
	return &kmsInfo{
		KMS:         h.keyManagement.KMS,
		Alg:         alg.String(),
		PendingAlg:  pendingAlg,
		AlgChangeAt: pendingEffective,
		RSAKeyLen:   rsaKeyLen,
		Rotation:    rotation,
	}, nil
}

// registerKeys wires routes for managing public keys and KMS-related endpoints.
func registerKeys(r fiber.Router, keyManagement KeyManagement, kvStorage smodel.KeyValueStore, storages smodel.Backends) {
	jwksH := &jwksHandlers{keyManagement: keyManagement}
	pkH := &publicKeyHandlers{
		apiManagedPKs: keyManagement.APIManagedPKs,
		storages:      storages,
		kvStorage:     kvStorage,
	}
	kmsH := &kmsHandlers{
		keyManagement: keyManagement,
		kvStorage:     kvStorage,
	}

	// Published JWKS
	r.Get("/entity-configuration/jwks", jwksH.getJWKS)

	// Public keys collection
	g := r.Group("/entity-configuration/keys")
	withCacheWipe := g.Use(entityConfigurationCacheInvalidationMiddleware)

	g.Get("/", pkH.list)
	withCacheWipe.Post("/", pkH.create)
	withCacheWipe.Put("/:kid", pkH.update)
	withCacheWipe.Post("/:kid", pkH.rotate)
	withCacheWipe.Delete("/:kid", pkH.delete)

	// KMS routes
	kmsWithCacheWipe := r.Use(entityConfigurationCacheInvalidationMiddleware)
	r.Get("/kms", kmsH.getInfo)
	kmsWithCacheWipe.Put("/kms/alg", kmsH.putAlg)
	kmsWithCacheWipe.Put("/kms/rsa-key-len", kmsH.putRSAKeyLen)
	r.Get("/kms/rotation", kmsH.getRotation)
	kmsWithCacheWipe.Put("/kms/rotation", kmsH.putRotation)
	kmsWithCacheWipe.Patch("/kms/rotation", kmsH.patchRotation)
	kmsWithCacheWipe.Post("/kms/rotate", kmsH.triggerRotate)
}
