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
		s := status.String()
		e.Status = &s
	}
}

// WithActor sets the event actor.
func WithActor(actor string) EventOption {
	return func(e *model.SubordinateEvent) {
		e.Actor = &actor
	}
}

// RecordEvent records an event using the provided event store and returns any error.
// This is designed for use within transactions where event recording failure
// should cause the entire transaction to roll back.
// Use the EventOption functions (WithStatus, WithMessage, WithActor) to configure the event.
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
