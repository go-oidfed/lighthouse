package adminapi

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// eventRecorder provides helper methods for recording subordinate events.
type eventRecorder struct {
	store model.SubordinateEventStore
}

// newEventRecorder creates a new eventRecorder.
func newEventRecorder(store model.SubordinateEventStore) *eventRecorder {
	return &eventRecorder{store: store}
}

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

// Record records a new event for a subordinate.
// Event recording failures are logged but do not fail the operation.
func (r *eventRecorder) Record(subordinateID uint, eventType string, opts ...EventOption) {
	if r.store == nil {
		return
	}

	event := model.SubordinateEvent{
		SubordinateID: subordinateID,
		Timestamp:     time.Now().Unix(),
		Type:          eventType,
	}

	for _, opt := range opts {
		opt(&event)
	}

	if err := r.store.Add(event); err != nil {
		log.Errorf("failed to record subordinate event: %v", err)
	}
}

// DeleteForSubordinate removes all events for a subordinate.
// This is called when a subordinate is deleted.
func (r *eventRecorder) DeleteForSubordinate(subordinateID uint) {
	if r.store == nil {
		return
	}

	if err := r.store.DeleteBySubordinateID(subordinateID); err != nil {
		log.Errorf("failed to delete subordinate events: %v", err)
	}
}
