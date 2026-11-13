package adminapi

import (
	"time"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// EventOption is a functional option for configuring an event.
type EventOption func(*model.SubordinateEvent)

// WithMessage sets the event message.
func WithMessage(msg string) EventOption {
	return func(e *model.SubordinateEvent) {
		e.Message = &msg
	}
}

// WithStatus sets the event status.
func WithStatus(status model.Status) EventOption {
	return func(e *model.SubordinateEvent) {
		e.Status = new(status.String())
	}
}

// WithActor sets the event actor.
func WithActor(actor string) EventOption {
	return func(e *model.SubordinateEvent) {
		e.Actor = &actor
	}
}

// WithEntityID sets the entity the event is about. Without it, the entity ID
// is taken from the subordinate; set it for an entity that is not a
// subordinate (subordinate ID 0).
func WithEntityID(entityID string) EventOption {
	return func(e *model.SubordinateEvent) {
		e.EntityID = entityID
	}
}

// RecordEvent records an event using the provided event store and returns any error.
// This is designed for use within transactions where event recording failure
// should cause the entire transaction to roll back.
// Use the EventOption functions (WithStatus, WithMessage, WithActor, WithEntityID) to configure the event.
func RecordEvent(
	store model.SubordinateEventStore,
	subordinateID uint,
	eventType string,
	opts ...EventOption,
) error {
	if store == nil {
		return nil
	}

	event := model.SubordinateEvent{
		SubordinateID: subordinateID,
		Timestamp:     time.Now().Unix(),
		Type:          eventType,
	}

	for _, opt := range opts {
		opt(&event)
	}

	return store.Add(event)
}
