package storage

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// newEventsTestStorage creates a unique in-memory SQLite database for event
// storage tests.
func newEventsTestStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", url.PathEscape(t.Name()))
	store, err := NewStorage(Config{Driver: DriverSQLite, DSN: dsn})
	require.NoError(t, err)
	return store
}

// addEventsTestSubordinate adds an active subordinate and returns its DB ID.
func addEventsTestSubordinate(t *testing.T, store *Storage, entityID string) uint {
	t.Helper()
	subs := store.SubordinateStorage()
	require.NoError(
		t, subs.Add(
			model.ExtendedSubordinateInfo{
				BasicSubordinateInfo: model.BasicSubordinateInfo{
					EntityID: entityID,
					Status:   model.StatusActive,
				},
			},
		),
	)
	info, err := subs.Get(entityID)
	require.NoError(t, err)
	require.NotNil(t, info)
	return info.ID
}

func eventTypes(events []model.SubordinateEvent) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}

func TestEventsAdd_FillsEntityIDFromSubordinate(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()

	activeID := addEventsTestSubordinate(t, store, "https://active.example.org")
	removedID := addEventsTestSubordinate(t, store, "https://removed.example.org")
	require.NoError(t, store.SubordinateStorage().DeleteByDBID(fmt.Sprint(removedID)))

	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: activeID, Timestamp: 1, Type: "a"}))
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: removedID, Timestamp: 2, Type: "b"}))
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: 9999, Timestamp: 3, Type: "c"}))

	var stored []model.SubordinateEvent
	require.NoError(t, store.DB().Order("id ASC").Find(&stored).Error)
	require.Len(t, stored, 3)
	require.Equal(t, "https://active.example.org", stored[0].EntityID)
	require.Equal(t, "https://removed.example.org", stored[1].EntityID, "removed subordinates count")
	require.Empty(t, stored[2].EntityID, "an unknown subordinate leaves the entity ID empty")
}

func TestEventsAdd_KeepsExplicitEntityID(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()

	subID := addEventsTestSubordinate(t, store, "https://sub.example.org")
	require.NoError(
		t, events.Add(
			model.SubordinateEvent{
				SubordinateID: subID,
				EntityID:      "https://explicit.example.org",
				Timestamp:     1,
				Type:          "a",
			},
		),
	)
	// An event about an entity that is not a subordinate.
	require.NoError(
		t, events.Add(model.SubordinateEvent{EntityID: "https://leaf.example.org", Timestamp: 2, Type: "b"}),
	)

	var stored []model.SubordinateEvent
	require.NoError(t, store.DB().Order("id ASC").Find(&stored).Error)
	require.Len(t, stored, 2)
	require.Equal(t, "https://explicit.example.org", stored[0].EntityID)
	require.Equal(t, "https://leaf.example.org", stored[1].EntityID)
	require.Zero(t, stored[1].SubordinateID)
}

func TestEventsAdd_FillsEntityIDInsideTransaction(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	backends, err := store.Backends(JTIStorageDB)
	require.NoError(t, err)

	err = backends.InTransaction(
		func(tx *model.Backends) error {
			if err := tx.Subordinates.Add(
				model.ExtendedSubordinateInfo{
					BasicSubordinateInfo: model.BasicSubordinateInfo{
						EntityID: "https://tx.example.org",
						Status:   model.StatusActive,
					},
				},
			); err != nil {
				return err
			}
			info, err := tx.Subordinates.Get("https://tx.example.org")
			if err != nil {
				return err
			}
			return tx.SubordinateEvents.Add(model.SubordinateEvent{SubordinateID: info.ID, Timestamp: 1, Type: "a"})
		},
	)
	require.NoError(t, err)

	entityID := "https://tx.example.org"
	got, _, err := store.SubordinateEventsStorage().Query(model.EventQueryOpts{EntityID: &entityID})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// TestEventsQuery_SpansMemberships: an entity's events stay together across
// removal and joining again.
func TestEventsQuery_SpansMemberships(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()
	const entityID = "https://member.example.org"

	firstID := addEventsTestSubordinate(t, store, entityID)
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: firstID, Timestamp: 10, Type: "created"}))
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: firstID, Timestamp: 20, Type: "updated"}))
	require.NoError(t, store.SubordinateStorage().DeleteByDBID(fmt.Sprint(firstID)))

	secondID := addEventsTestSubordinate(t, store, entityID)
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: secondID, Timestamp: 30, Type: "created"}))
	// Same timestamp as the previous event: the ID breaks the tie.
	require.NoError(
		t, events.Add(model.SubordinateEvent{SubordinateID: secondID, Timestamp: 30, Type: "status_updated"}),
	)

	other := addEventsTestSubordinate(t, store, "https://other.example.org")
	require.NoError(t, events.Add(model.SubordinateEvent{SubordinateID: other, Timestamp: 40, Type: "created"}))

	entity := entityID
	got, total, err := events.Query(model.EventQueryOpts{EntityID: &entity})
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Equal(t, []string{"status_updated", "created", "updated", "created"}, eventTypes(got))
	for _, e := range got {
		require.Equal(t, entityID, e.EntityID)
	}
}

func TestEventsQuery_Filters(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()

	alice := "alice"
	bob := "bob"
	add := func(entityID string, ts int64, typ string, actor *string) {
		t.Helper()
		require.NoError(
			t, events.Add(model.SubordinateEvent{EntityID: entityID, Timestamp: ts, Type: typ, Actor: actor}),
		)
	}
	add("https://a.example.org", 100, "created", &alice)
	add("https://a.example.org", 200, "deleted", &bob)
	add("https://b.example.org", 300, "created", &bob)
	add("https://b.example.org", 400, "jwks_replaced", nil)
	add("https://c.example.org", 500, "status_updated", &alice)

	t.Run(
		"All", func(t *testing.T) {
			got, total, err := events.Query(model.EventQueryOpts{})
			require.NoError(t, err)
			require.EqualValues(t, 5, total)
			require.Equal(
				t, []string{"status_updated", "jwks_replaced", "created", "deleted", "created"},
				eventTypes(got),
			)
		},
	)
	t.Run(
		"TypesOR", func(t *testing.T) {
			got, total, err := events.Query(model.EventQueryOpts{EventTypes: []string{"deleted", "jwks_replaced"}})
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			require.Equal(t, []string{"jwks_replaced", "deleted"}, eventTypes(got))
		},
	)
	t.Run(
		"SingleEventType", func(t *testing.T) {
			typ := "created"
			got, total, err := events.Query(model.EventQueryOpts{EventType: &typ})
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			require.Equal(t, []string{"created", "created"}, eventTypes(got))
		},
	)
	t.Run(
		"Entity", func(t *testing.T) {
			entity := "https://b.example.org"
			got, total, err := events.Query(model.EventQueryOpts{EntityID: &entity})
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			require.Equal(t, []string{"jwks_replaced", "created"}, eventTypes(got))
		},
	)
	t.Run(
		"Actor", func(t *testing.T) {
			got, total, err := events.Query(model.EventQueryOpts{Actor: &alice})
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			require.Equal(t, []string{"status_updated", "created"}, eventTypes(got))
		},
	)
	t.Run(
		"TimeRange", func(t *testing.T) {
			from := int64(200)
			to := int64(400)
			got, total, err := events.Query(model.EventQueryOpts{FromTime: &from, ToTime: &to})
			require.NoError(t, err)
			require.EqualValues(t, 3, total)
			require.Equal(t, []string{"jwks_replaced", "created", "deleted"}, eventTypes(got))
		},
	)
	t.Run(
		"Paging", func(t *testing.T) {
			got, total, err := events.Query(model.EventQueryOpts{Limit: 2, Offset: 2})
			require.NoError(t, err)
			require.EqualValues(t, 5, total)
			require.Equal(t, []string{"created", "deleted"}, eventTypes(got))
		},
	)
	t.Run(
		"Combined", func(t *testing.T) {
			entity := "https://a.example.org"
			got, total, err := events.Query(
				model.EventQueryOpts{EntityID: &entity, Actor: &bob, EventTypes: []string{"deleted", "created"}},
			)
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Equal(t, []string{"deleted"}, eventTypes(got))
		},
	)
}

func TestEventsQuery_LimitCaps(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()

	subID := addEventsTestSubordinate(t, store, "https://many.example.org")
	batch := make([]model.SubordinateEvent, 150)
	for i := range batch {
		batch[i] = model.SubordinateEvent{
			SubordinateID: subID, EntityID: "https://many.example.org", Timestamp: int64(i), Type: "updated",
		}
	}
	require.NoError(t, store.DB().Create(&batch).Error)

	got, total, err := events.Query(model.EventQueryOpts{})
	require.NoError(t, err)
	require.EqualValues(t, 150, total)
	require.Len(t, got, 50, "default page size")

	got, _, err = events.Query(model.EventQueryOpts{Limit: 1_000_000})
	require.NoError(t, err)
	require.Len(t, got, 150)

	got, _, err = events.GetBySubordinateID(subID, model.EventQueryOpts{Limit: 10000})
	require.NoError(t, err)
	require.Len(t, got, 100, "GetBySubordinateID keeps its 100 cap")
}

func TestEventsRemovedEntityIDs(t *testing.T) {
	t.Parallel()
	store := newEventsTestStorage(t)
	events := store.SubordinateEventsStorage()
	add := func(entityID string) {
		t.Helper()
		require.NoError(t, events.Add(model.SubordinateEvent{EntityID: entityID, Timestamp: 1, Type: "created"}))
	}
	add("https://gone.example")
	add("https://gone.example")
	add("https://GONE-two.example")
	add("https://other.example")
	add("https://100%25.example")
	// Current subordinates, including one that was removed and joined
	// again, are not removed.
	addEventsTestSubordinate(t, store, "https://gone-but-here.example")
	add("https://gone-but-here.example")
	again := addEventsTestSubordinate(t, store, "https://gone-again.example")
	add("https://gone-again.example")
	require.NoError(t, store.SubordinateStorage().DeleteByDBID(fmt.Sprint(again)))
	addEventsTestSubordinate(t, store, "https://gone-again.example")
	// A removed (soft-deleted) subordinate is removed.
	deleted := addEventsTestSubordinate(t, store, "https://gone-deleted.example")
	add("https://gone-deleted.example")
	require.NoError(t, store.SubordinateStorage().DeleteByDBID(fmt.Sprint(deleted)))

	got, err := events.RemovedEntityIDs("GoNe", 10)
	require.NoError(t, err)
	require.Equal(
		t, []string{"https://GONE-two.example", "https://gone-deleted.example", "https://gone.example"}, got,
		"sorted, distinct, case-insensitive",
	)

	got, err = events.RemovedEntityIDs("gone", 1)
	require.NoError(t, err)
	require.Len(t, got, 1)

	got, err = events.RemovedEntityIDs("", 10)
	require.NoError(t, err)
	require.Len(t, got, 5, "an empty search lists all")

	// LIKE wildcards in the search match literally.
	got, err = events.RemovedEntityIDs("%", 10)
	require.NoError(t, err)
	require.Equal(t, []string{"https://100%25.example"}, got)
	got, err = events.RemovedEntityIDs("_", 10)
	require.NoError(t, err)
	require.Empty(t, got)
}
