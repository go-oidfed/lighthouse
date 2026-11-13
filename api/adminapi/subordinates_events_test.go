package adminapi

import (
	"errors"
	"testing"
	"time"

	smodel "github.com/go-oidfed/lighthouse/storage/model"
)

type mockSubordinateEventStore struct {
	addFn func(smodel.SubordinateEvent) error
}

func (m *mockSubordinateEventStore) Add(event smodel.SubordinateEvent) error {
	return m.addFn(event)
}

func (*mockSubordinateEventStore) GetBySubordinateID(_ uint, _ smodel.EventQueryOpts) ([]smodel.SubordinateEvent, int64, error) {
	return nil, 0, nil
}

func (*mockSubordinateEventStore) Query(_ smodel.EventQueryOpts) ([]smodel.SubordinateEvent, int64, error) {
	return nil, 0, nil
}

func (*mockSubordinateEventStore) RemovedEntityIDs(_ string, _ int) ([]string, error) {
	return nil, nil
}

func TestEventOptions(t *testing.T) {
	t.Parallel()

	event := &smodel.SubordinateEvent{}
	WithMessage("updated subordinate")(event)
	WithStatus(smodel.StatusActive)(event)
	WithActor("admin@example.com")(event)
	WithEntityID("https://sub.example.com")(event)

	if event.EntityID != "https://sub.example.com" {
		t.Fatalf("unexpected entity ID option result: %+v", event)
	}
	if event.Message == nil || *event.Message != "updated subordinate" {
		t.Fatalf("unexpected message option result: %+v", event)
	}
	if event.Status == nil || *event.Status != smodel.StatusActive.String() {
		t.Fatalf("unexpected status option result: %+v", event)
	}
	if event.Actor == nil || *event.Actor != "admin@example.com" {
		t.Fatalf("unexpected actor option result: %+v", event)
	}
}

func TestRecordEvent(t *testing.T) {
	t.Parallel()

	t.Run("NilStoreIsNoop", func(t *testing.T) {
		t.Parallel()

		if err := RecordEvent(nil, 7, smodel.EventTypeCreated); err != nil {
			t.Fatalf("expected nil error for nil store, got %v", err)
		}
	})

	t.Run("AddsEventWithOptions", func(t *testing.T) {
		t.Parallel()

		var got smodel.SubordinateEvent
		store := &mockSubordinateEventStore{
			addFn: func(event smodel.SubordinateEvent) error {
				got = event
				return nil
			},
		}
		before := time.Now().Unix()

		err := RecordEvent(
			store,
			9,
			smodel.EventTypeUpdated,
			WithStatus(smodel.StatusBlocked),
			WithMessage("blocked by admin"),
			WithActor("alice"),
		)
		after := time.Now().Unix()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got.SubordinateID != 9 || got.Type != smodel.EventTypeUpdated {
			t.Fatalf("unexpected event core fields: %+v", got)
		}
		if got.Timestamp < before || got.Timestamp > after {
			t.Fatalf("expected timestamp between %d and %d, got %d", before, after, got.Timestamp)
		}
		if got.Status == nil || *got.Status != smodel.StatusBlocked.String() {
			t.Fatalf("unexpected event status: %+v", got)
		}
		if got.Message == nil || *got.Message != "blocked by admin" {
			t.Fatalf("unexpected event message: %+v", got)
		}
		if got.Actor == nil || *got.Actor != "alice" {
			t.Fatalf("unexpected event actor: %+v", got)
		}
	})

	t.Run("PropagatesStoreError", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("write failed")
		store := &mockSubordinateEventStore{
			addFn: func(_ smodel.SubordinateEvent) error {
				return wantErr
			},
		}

		err := RecordEvent(store, 3, smodel.EventTypeDeleted)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}
	})
}
