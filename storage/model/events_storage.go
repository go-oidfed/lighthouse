package model

// SubordinateEventStore is an interface for storing and retrieving subordinate events.
type SubordinateEventStore interface {
	// Add creates a new event record. When EntityID is empty and
	// SubordinateID is set, EntityID is filled from the subordinate
	// (including a removed one).
	Add(event SubordinateEvent) error

	// GetBySubordinateID returns events for a subordinate with optional filtering and pagination.
	GetBySubordinateID(subordinateID uint, opts EventQueryOpts) ([]SubordinateEvent, int64, error)

	// Query returns events of all entities matching opts, newest first, and
	// the total count for pagination.
	Query(opts EventQueryOpts) ([]SubordinateEvent, int64, error)

	// RemovedEntityIDs returns up to limit distinct entity IDs, sorted, that
	// contain search (case-insensitively) and have events but are not a
	// current subordinate.
	RemovedEntityIDs(search string, limit int) ([]string, error)
}

// EventQueryOpts contains options for querying events.
type EventQueryOpts struct {
	// Limit is the maximum number of events to return (default: 50; max: 100
	// for GetBySubordinateID, 10000 for Query).
	Limit int
	// Offset is the number of events to skip for pagination.
	Offset int
	// EventType filters events by type (e.g., "created", "deleted").
	EventType *string
	// EventTypes filters events to any of these types.
	EventTypes []string
	// EntityID filters events by the entity they are about.
	EntityID *string
	// Actor filters events by actor.
	Actor *string
	// FromTime filters events with timestamp >= this value (unix seconds).
	FromTime *int64
	// ToTime filters events with timestamp <= this value (unix seconds).
	ToTime *int64
}
