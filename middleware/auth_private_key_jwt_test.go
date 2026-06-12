package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage"
)

func TestJTICacheStorage(t *testing.T) {
	t.Run("Exists returns false for new JTI", func(t *testing.T) {
		s := storage.NewJTICacheStorage()
		exists, err := s.Exists("test-jti-123")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Store and Exists", func(t *testing.T) {
		s := storage.NewJTICacheStorage()
		jti := "test-jti-456"
		expiresAt := time.Now().Add(10 * time.Minute)

		err := s.Store(jti, expiresAt)
		require.NoError(t, err)

		exists, err := s.Exists(jti)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Store with expired TTL doesn't store", func(t *testing.T) {
		s := storage.NewJTICacheStorage()
		jti := "test-jti-789"
		expiresAt := time.Now().Add(-1 * time.Minute)

		err := s.Store(jti, expiresAt)
		require.NoError(t, err)

		// Should not be stored since it's already expired
		exists, err := s.Exists(jti)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Cleanup is no-op", func(t *testing.T) {
		s := storage.NewJTICacheStorage()
		err := s.Cleanup()
		assert.NoError(t, err)
	})
}

func TestPrivateKeyJWTAuth_ErrorTypes(t *testing.T) {
	t.Run("ErrInvalidRequest creates proper error", func(t *testing.T) {
		err := ErrInvalidRequest("test message")
		authErr, ok := err.(*AuthError)
		require.True(t, ok)
		assert.Equal(t, "invalid_request", authErr.Code)
		assert.Equal(t, "test message", authErr.Description)
	})

	t.Run("ErrInvalidClient creates proper error", func(t *testing.T) {
		err := ErrInvalidClient("test message")
		authErr, ok := err.(*AuthError)
		require.True(t, ok)
		assert.Equal(t, "invalid_client", authErr.Code)
		assert.Equal(t, "test message", authErr.Description)
	})

	t.Run("ErrInvalidGrant creates proper error", func(t *testing.T) {
		err := ErrInvalidGrant("test message")
		authErr, ok := err.(*AuthError)
		require.True(t, ok)
		assert.Equal(t, "invalid_grant", authErr.Code)
		assert.Equal(t, "test message", authErr.Description)
	})

	t.Run("ErrServerError creates proper error", func(t *testing.T) {
		err := ErrServerError("test message")
		authErr, ok := err.(*AuthError)
		require.True(t, ok)
		assert.Equal(t, "server_error", authErr.Code)
		assert.Equal(t, "test message", authErr.Description)
	})
}
