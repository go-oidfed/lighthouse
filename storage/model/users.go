package model

import (
	"time"
)

// User represents an admin user that can access the admin API.
// When no users exist, the admin API is open; when one or more users exist,
// only authenticated users may access it.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Username is unique identifier for login
	Username string `gorm:"uniqueIndex" json:"username"`
	// PasswordHash stores a PHC-formatted argon2id hash of the user's password
	PasswordHash string `json:"-"`
	// DisplayName is optional, for UI friendliness
	DisplayName string `json:"display_name"`
	// Disabled allows soft-disable of a user without deletion
	Disabled bool `json:"disabled"`
}

// UsersStore abstracts CRUD and authentication helpers for admin users.
type UsersStore interface {
	// Count returns the number of users present in the store
	Count() (int64, error)
	// List returns all users (without password hashes)
	List() ([]User, error)
	// Get returns a user by username
	Get(username string) (*User, error)
	// Create creates a user; the implementation must hash the password
	Create(username, password, displayName string) (*User, error)
	// Update updates username/display name and optionally password
	Update(username string, displayName *string, newPassword *string, disabled *bool) (*User, error)
	// Delete deletes a user by username
	Delete(username string) error
	// Authenticate checks a username/password combo and returns the user
	Authenticate(username, password string) (*User, error)
}
