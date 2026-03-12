package model

// SubordinateEventStore is an interface for storing and retrieving subordinate events.
type SubordinateEventStore interface {
	// Add creates a new event record.
	Add(event SubordinateEvent) error

	// GetBySubordinateID returns events for a subordinate with optional filtering and pagination.
	GetBySubordinateID(subordinateID uint, opts EventQueryOpts) ([]SubordinateEvent, int64, error)

	// DeleteBySubordinateID removes all events for a subordinate (used on subordinate deletion).
	DeleteBySubordinateID(subordinateID uint) error
}

// EventQueryOpts contains options for querying events.
type EventQueryOpts struct {
	// Limit is the maximum number of events to return (default: 50, max: 100).
	Limit int
	// Offset is the number of events to skip for pagination.
	Offset int
	// EventType filters events by type (e.g., "created", "deleted").
	EventType *string
	// FromTime filters events with timestamp >= this value (unix seconds).
	FromTime *int64
	// ToTime filters events with timestamp <= this value (unix seconds).
	ToTime *int64
}
