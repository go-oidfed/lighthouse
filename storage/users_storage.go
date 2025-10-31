package storage

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// UsersStorage returns a UsersStorage
func (s *Storage) UsersStorage() *UsersStorage {
	return &UsersStorage{db: s.db, params: s.userParams}
}

// UsersStorage implements UsersStore using GORM
type UsersStorage struct {
	db     *gorm.DB
	params Argon2idParams
}

// Count returns the number of users present in the store
func (s *UsersStorage) Count() (int64, error) {
	var count int64
	if err := s.db.Model(&model.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// List returns all users (without password hashes)
func (s *UsersStorage) List() ([]model.User, error) {
	var users []model.User
	if err := s.db.Model(&model.User{}).Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		users[i].PasswordHash = ""
	}
	return users, nil
}

// Get returns a user by username
func (s *UsersStorage) Get(username string) (*model.User, error) {
	var u model.User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, model.NotFoundErrorFmt("user not found: %s", username)
	}
	u.PasswordHash = ""
	return &u, nil
}

// Create creates a user with an Argon2id-hashed password
func (s *UsersStorage) Create(username, password, displayName string) (*model.User, error) {
	if len(username) == 0 || len(password) == 0 {
		return nil, errors.Errorf("username and password are required")
	}
	var existing int64
	if err := s.db.Model(&model.User{}).Where("username = ?", username).Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, model.AlreadyExistsErrorFmt("user already exists: %s", username)
	}
	hash, err := hashPasswordArgon2id(password, s.params)
	if err != nil {
		return nil, err
	}
	u := model.User{Username: username, PasswordHash: hash, DisplayName: displayName}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return &u, nil
}

// Update updates display name / password / disabled
func (s *UsersStorage) Update(username string, displayName *string, newPassword *string, disabled *bool) (*model.User, error) {
	var u model.User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, model.NotFoundErrorFmt("user not found: %s", username)
	}
	if displayName != nil {
		u.DisplayName = *displayName
	}
	if disabled != nil {
		u.Disabled = *disabled
	}
	if newPassword != nil {
		if len(*newPassword) == 0 {
			return nil, errors.Errorf("password cannot be empty")
		}
		hash, err := hashPasswordArgon2id(*newPassword, s.params)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = hash
	}
	if err := s.db.Save(&u).Error; err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return &u, nil
}

// Delete deletes a user by username
func (s *UsersStorage) Delete(username string) error {
	res := s.db.Where("username = ?", username).Delete(&model.User{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.NotFoundErrorFmt("user not found: %s", username)
	}
	return nil
}

// Authenticate validates username/password and auto-upgrades hash if params changed
func (s *UsersStorage) Authenticate(username, password string) (*model.User, error) {
	var u model.User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, model.NotFoundErrorFmt("user not found: %s", username)
	}
	if u.Disabled {
		return nil, errors.Errorf("user disabled")
	}
	ok, err := verifyPasswordArgon2id(u.PasswordHash, password)
	if err != nil || !ok {
		return nil, errors.Errorf("invalid credentials")
	}
	if stored, err := extractArgon2idParams(u.PasswordHash); err == nil && !argon2idParamsEqual(stored, s.params) {
		if newHash, err := hashPasswordArgon2id(password, s.params); err == nil {
			_ = s.db.Model(&model.User{}).Where("id = ?", u.ID).Update("password_hash", newHash).Error
		}
	}
	u.PasswordHash = ""
	return &u, nil
}

// hashPasswordArgon2id returns a PHC-formatted argon2id hash string
// Format: $argon2id$v=19$m=65536,t=1,p=4$<saltB64>$<hashB64>
func hashPasswordArgon2id(password string, p Argon2idParams) (string, error) {
	if p.Time == 0 {
		p = defaultArgon2idParams()
	}
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Parallelism, p.KeyLen)
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(dk)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", p.MemoryKiB, p.Time, p.Parallelism, saltB64, hashB64), nil
}

// verifyPasswordArgon2id verifies the given password against a PHC-formatted argon2id hash
func verifyPasswordArgon2id(encoded, password string) (bool, error) {
	params, salt, hash, err := parseArgon2id(encoded)
	if err != nil {
		return false, err
	}
	dk := argon2.IDKey([]byte(password), salt, params.Time, params.MemoryKiB, params.Parallelism, uint32(len(hash)))
	if subtle.ConstantTimeCompare(dk, hash) == 1 {
		return true, nil
	}
	return false, nil
}

// extractArgon2idParams parses a PHC-formatted argon2id string and returns parameters
func extractArgon2idParams(encoded string) (Argon2idParams, error) {
	p, _, _, err := parseArgon2id(encoded)
	return p, err
}

// parseArgon2id parses a PHC-formatted argon2id hash and returns parameters, salt and hash bytes.
func parseArgon2id(encoded string) (Argon2idParams, []byte, []byte, error) {
	var out Argon2idParams
	if !strings.HasPrefix(encoded, "$argon2id$") {
		return out, nil, nil, errors.Errorf("unsupported password hash format")
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return out, nil, nil, errors.Errorf("invalid argon2id hash format")
	}
	if parts[2] != "v=19" {
		return out, nil, nil, errors.Errorf("unsupported argon2 version")
	}
	for _, kv := range strings.Split(parts[3], ",") {
		if strings.HasPrefix(kv, "m=") {
			v, err := strconv.ParseUint(strings.TrimPrefix(kv, "m="), 10, 32)
			if err != nil {
				return out, nil, nil, err
			}
			out.MemoryKiB = uint32(v)
		} else if strings.HasPrefix(kv, "t=") {
			v, err := strconv.ParseUint(strings.TrimPrefix(kv, "t="), 10, 32)
			if err != nil {
				return out, nil, nil, err
			}
			out.Time = uint32(v)
		} else if strings.HasPrefix(kv, "p=") {
			v, err := strconv.ParseUint(strings.TrimPrefix(kv, "p="), 10, 8)
			if err != nil {
				return out, nil, nil, err
			}
			out.Parallelism = uint8(v)
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return out, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return out, nil, nil, err
	}
	out.SaltLen = uint32(len(salt))
	out.KeyLen = uint32(len(hash))
	return out, salt, hash, nil
}

func argon2idParamsEqual(a, b Argon2idParams) bool {
	return a.Time == b.Time && a.MemoryKiB == b.MemoryKiB && a.Parallelism == b.Parallelism && a.KeyLen == b.KeyLen && a.SaltLen == b.SaltLen
}

func defaultArgon2idParams() Argon2idParams {
	return Argon2idParams{Time: 1, MemoryKiB: 64 * 1024, Parallelism: 4, KeyLen: 32, SaltLen: 16}
}
