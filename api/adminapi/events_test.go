package adminapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

type eventsListResult struct {
	Events     []map[string]any `json:"events"`
	Pagination struct {
		Total  int64 `json:"total"`
		Limit  int   `json:"limit"`
		Offset int   `json:"offset"`
	} `json:"pagination"`
}

// setupEventsApp registers the base subordinate endpoints and the events
// query on one app over fresh storage.
func setupEventsApp(t *testing.T) (*fiber.App, testBackends) {
	t.Helper()
	app, backends := setupSubordinateBaseApp(t)
	registerEvents(app, backends.SubordinateEvents)
	return app, backends
}

func getEvents(t *testing.T, app *fiber.App, url string) eventsListResult {
	t.Helper()
	resp, body := doRequest(t, app, httptest.NewRequest("GET", url, http.NoBody))
	requireStatus(t, resp, body, http.StatusOK)
	var res eventsListResult
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("Failed to parse response: %v (%s)", err, body)
	}
	return res
}

func addEvent(t *testing.T, store model.SubordinateEventStore, e model.SubordinateEvent) {
	t.Helper()
	if err := store.Add(e); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}
}

func eventTimestamps(events []map[string]any) []int64 {
	out := make([]int64, len(events))
	for i, e := range events {
		out[i] = int64(e["timestamp"].(float64))
	}
	return out
}

func TestEvents_Filters(t *testing.T) {
	t.Parallel()
	app, b := setupEventsApp(t)
	alice, bob := "alice@example.com", "bob@example.com"
	for _, e := range []model.SubordinateEvent{
		{EntityID: "https://a.example.org", Timestamp: 100, Type: model.EventTypeCreated, Actor: &alice},
		{EntityID: "https://a.example.org", Timestamp: 200, Type: model.EventTypeStatusUpdated, Actor: &alice},
		{EntityID: "https://a.example.org", Timestamp: 300, Type: model.EventTypeDeleted, Actor: &bob},
		{EntityID: "https://b.example.org", Timestamp: 250, Type: model.EventTypeStatusUpdated, Actor: &alice},
	} {
		addEvent(t, b.SubordinateEvents, e)
	}

	tests := []struct {
		name string
		url  string
		want []int64
	}{
		{"None/NewestFirst", "/events", []int64{300, 250, 200, 100}},
		{"EntityID", "/events?entity_id=https://b.example.org", []int64{250}},
		{"SingleType", "/events?type=status_updated", []int64{250, 200}},
		{"RepeatedTypesMatchAny", "/events?type=created&type=deleted", []int64{300, 100}},
		{"Actor", "/events?actor=bob@example.com", []int64{300}},
		{"From", "/events?from=250", []int64{300, 250}},
		{"To", "/events?to=200", []int64{200, 100}},
		{
			"Combined",
			"/events?entity_id=https://a.example.org&type=created&type=status_updated&actor=alice@example.com&from=150&to=260",
			[]int64{200},
		},
		{"NoMatch", "/events?entity_id=https://none.example.org", []int64{}},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				res := getEvents(t, app, tt.url)
				if res.Events == nil {
					t.Fatalf("events must be an array")
				}
				if got := eventTimestamps(res.Events); fmt.Sprint(got) != fmt.Sprint(tt.want) {
					t.Errorf("timestamps = %v, want %v", got, tt.want)
				}
				if res.Pagination.Total != int64(len(tt.want)) {
					t.Errorf("total = %d, want %d", res.Pagination.Total, len(tt.want))
				}
			},
		)
	}
}

func TestEvents_Paging(t *testing.T) {
	t.Parallel()
	app, b := setupEventsApp(t)
	for i := range 5 {
		addEvent(
			t, b.SubordinateEvents,
			model.SubordinateEvent{EntityID: "https://p.example.org", Timestamp: int64(100 + i), Type: "updated"},
		)
	}

	res := getEvents(t, app, "/events?limit=2&offset=1")
	if res.Pagination.Total != 5 || res.Pagination.Limit != 2 || res.Pagination.Offset != 1 {
		t.Fatalf("pagination = %+v", res.Pagination)
	}
	if got := eventTimestamps(res.Events); fmt.Sprint(got) != "[103 102]" {
		t.Errorf("page = %v", got)
	}

	if res := getEvents(t, app, "/events"); res.Pagination.Limit != 50 || res.Pagination.Offset != 0 {
		t.Errorf("default pagination = %+v", res.Pagination)
	}
	if res := getEvents(t, app, "/events?limit=0"); res.Pagination.Limit != 50 {
		t.Errorf("limit=0 = %d, want the default 50", res.Pagination.Limit)
	}
	if res := getEvents(t, app, "/events?limit=999999"); res.Pagination.Limit != 10000 || len(res.Events) != 5 {
		t.Errorf("clamped limit = %d (%d events), want 10000", res.Pagination.Limit, len(res.Events))
	}
	if res := getEvents(t, app, "/events?offset=5"); len(res.Events) != 0 || res.Pagination.Total != 5 {
		t.Errorf("offset past the end = %+v", res)
	}
}

func TestEvents_BadParams(t *testing.T) {
	t.Parallel()
	app, _ := setupEventsApp(t)
	for _, url := range []string{
		"/events?limit=abc",
		"/events?limit=-1",
		"/events?offset=-1",
		"/events?offset=x",
		"/events?from=yesterday",
		"/events?to=1.5",
		"/events/removed-entities?limit=abc",
		"/events/removed-entities?limit=-5",
	} {
		resp, body := doRequest(t, app, httptest.NewRequest("GET", url, http.NoBody))
		assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")
	}
}

// TestEvents_RemovedSubordinateKeepsHistory: deleting a subordinate keeps
// its events and records the removal, and the events query still returns
// them by entity ID.
func TestEvents_RemovedSubordinateKeepsHistory(t *testing.T) {
	t.Parallel()
	backends, err := newSubordinateTestStorage(t).Backends(storage.JTIStorageDB)
	if err != nil {
		t.Fatalf("Failed to create backends: %v", err)
	}
	app := fiber.New()
	app.Use(actorMiddleware(ActorConfig{Source: ActorSourceHeader}))
	registerSubordinatesBase(app, backends, nil)
	registerEvents(app, backends.SubordinateEvents)

	const entityID = "https://removed.example.org"
	id := addSubordinate(t, backends, entityID, model.StatusActive)
	addEvent(t, backends.SubordinateEvents, model.SubordinateEvent{SubordinateID: id, Timestamp: 1, Type: "updated"})

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d", id), http.NoBody)
	req.Header.Set("X-Actor", "operator@example.org")
	resp, body := doRequest(t, app, req)
	requireStatus(t, resp, body, http.StatusNoContent)

	res := getEvents(t, app, "/events?entity_id="+entityID)
	if len(res.Events) != 2 || res.Pagination.Total != 2 {
		t.Fatalf("expected the kept event and the removal, got %+v", res)
	}
	removal, kept := res.Events[0], res.Events[1]
	if removal["type"] != model.EventTypeDeleted {
		t.Errorf("newest event type = %v, want %q", removal["type"], model.EventTypeDeleted)
	}
	if removal["actor"] != "operator@example.org" {
		t.Errorf("removal actor = %v", removal["actor"])
	}
	if removal["message"] != "subordinate deleted: "+entityID {
		t.Errorf("removal message = %v", removal["message"])
	}
	if removal["status"] != model.StatusActive.String() {
		t.Errorf("removal status = %v", removal["status"])
	}
	for _, e := range res.Events {
		if e["entity_id"] != entityID || e["subordinate_id"].(float64) != float64(id) {
			t.Errorf("event subject = %v / %v", e["entity_id"], e["subordinate_id"])
		}
	}
	if kept["type"] != "updated" {
		t.Errorf("kept event type = %v", kept["type"])
	}

	// The removal's type filter finds it among all events.
	if res := getEvents(t, app, "/events?type=deleted"); len(res.Events) != 1 {
		t.Errorf("type=deleted = %+v", res.Events)
	}
	// It is listed among the removed entities.
	if got := getRemovedEntities(t, app, "/events/removed-entities"); fmt.Sprint(got) != "["+entityID+"]" {
		t.Errorf("removed entities = %v", got)
	}
}

// TestEvents_NonSubordinateEntity: an event about an entity that is not a
// subordinate is stored and returned with subordinate_id 0.
func TestEvents_NonSubordinateEntity(t *testing.T) {
	t.Parallel()
	app, b := setupEventsApp(t)
	if err := RecordEvent(
		b.SubordinateEvents, 0, "trust_mark_issued",
		WithEntityID("https://leaf.example.org"), WithMessage("issued"),
	); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	res := getEvents(t, app, "/events?entity_id=https://leaf.example.org")
	if len(res.Events) != 1 {
		t.Fatalf("expected 1 event, got %+v", res)
	}
	ev := res.Events[0]
	if ev["subordinate_id"].(float64) != 0 {
		t.Errorf("subordinate_id = %v, want 0", ev["subordinate_id"])
	}
	if ev["entity_id"] != "https://leaf.example.org" {
		t.Errorf("entity_id = %v", ev["entity_id"])
	}
	if id, ok := ev["id"].(float64); !ok || id == 0 {
		t.Errorf("id missing: %+v", ev)
	}
	if ev["type"] != "trust_mark_issued" || ev["message"] != "issued" {
		t.Errorf("unexpected event: %+v", ev)
	}
}

func getRemovedEntities(t *testing.T, app *fiber.App, url string) []string {
	t.Helper()
	resp, body := doRequest(t, app, httptest.NewRequest("GET", url, http.NoBody))
	requireStatus(t, resp, body, http.StatusOK)
	var res struct {
		EntityIDs []string `json:"entity_ids"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatal(err)
	}
	if res.EntityIDs == nil {
		t.Fatalf("entity_ids must be an array: %s", body)
	}
	return res.EntityIDs
}

func TestEvents_RemovedEntities(t *testing.T) {
	t.Parallel()
	app, b := setupEventsApp(t)
	addSubordinate(t, b.Backends, "https://current.example.org", model.StatusActive)
	for _, id := range []string{
		"https://current.example.org", "https://gone-b.example.org", "https://gone-a.example.org", "https://other.test",
	} {
		addEvent(t, b.SubordinateEvents, model.SubordinateEvent{EntityID: id, Timestamp: 1, Type: model.EventTypeCreated})
	}

	if got := getRemovedEntities(t, app, "/events/removed-entities"); fmt.Sprint(got) !=
		"[https://gone-a.example.org https://gone-b.example.org https://other.test]" {
		t.Errorf("all = %v", got)
	}
	if got := getRemovedEntities(t, app, "/events/removed-entities?q=GONE"); len(got) != 2 {
		t.Errorf("q=GONE = %v", got)
	}
	if got := getRemovedEntities(t, app, "/events/removed-entities?limit=1"); len(got) != 1 {
		t.Errorf("limit=1 = %v", got)
	}
	if got := getRemovedEntities(t, app, "/events/removed-entities?q=nomatch"); len(got) != 0 {
		t.Errorf("nomatch = %v", got)
	}
}

// TestSubordinateHistory_Unchanged pins the per-subordinate history route:
// its fields, filters, paging and limit cap stay as they were, it ignores the
// events query's entity_id and actor filters, and a removed subordinate's
// history is not found there. Events gain id, entity_id and subordinate_id.
func TestSubordinateHistory_Unchanged(t *testing.T) {
	t.Parallel()
	app, b := setupSubordinateBaseApp(t)
	const entityID = "https://pinned.example.org"
	id := addSubordinate(t, b.Backends, entityID, model.StatusPending)
	other := addSubordinate(t, b.Backends, "https://other.example.org", model.StatusPending)
	actor := "admin"
	for i := range 120 {
		addEvent(
			t, b.SubordinateEvents, model.SubordinateEvent{
				SubordinateID: id, Timestamp: int64(1000 + i), Type: model.EventTypeUpdated,
				Status: new("pending"), Message: new(fmt.Sprintf("m%d", i)), Actor: &actor,
			},
		)
	}
	addEvent(t, b.SubordinateEvents, model.SubordinateEvent{SubordinateID: id, Timestamp: 5, Type: model.EventTypeCreated})
	addEvent(t, b.SubordinateEvents, model.SubordinateEvent{SubordinateID: other, Timestamp: 6, Type: model.EventTypeCreated})
	// An event about the same entity that is not linked to the subordinate
	// row is not part of its history.
	addEvent(t, b.SubordinateEvents, model.SubordinateEvent{EntityID: entityID, Timestamp: 7, Type: "unlinked"})

	history := func(query string) eventsListResult {
		t.Helper()
		return getEvents(t, app, fmt.Sprintf("/subordinates/%d/history%s", id, query))
	}

	res := history("")
	if res.Pagination.Total != 121 || res.Pagination.Limit != 50 || res.Pagination.Offset != 0 || len(res.Events) != 50 {
		t.Fatalf("default page = %+v (%d events)", res.Pagination, len(res.Events))
	}
	first := res.Events[0]
	want := map[string]any{
		"timestamp": float64(1119), "type": "updated", "status": "pending", "message": "m119", "actor": "admin",
	}
	for k, v := range want {
		if first[k] != v {
			t.Errorf("newest event %s = %v, want %v", k, first[k], v)
		}
	}

	if res := history("?limit=1000"); res.Pagination.Limit != 100 || len(res.Events) != 100 {
		t.Errorf("limit cap = %d (%d events), want 100", res.Pagination.Limit, len(res.Events))
	}
	if res := history("?limit=-3"); res.Pagination.Limit != 50 || len(res.Events) != 50 {
		t.Errorf("negative limit = %d (%d events), want the default 50", res.Pagination.Limit, len(res.Events))
	}
	if res := history("?type=created"); res.Pagination.Total != 1 || res.Events[0]["timestamp"].(float64) != 5 {
		t.Errorf("type filter = %+v", res)
	} else {
		for _, k := range []string{"status", "message", "actor"} {
			if _, ok := res.Events[0][k]; ok {
				t.Errorf("unset %s must be omitted: %+v", k, res.Events[0])
			}
		}
	}
	if res := history("?from=1118&to=1119"); fmt.Sprint(eventTimestamps(res.Events)) != "[1119 1118]" {
		t.Errorf("time range = %v", eventTimestamps(res.Events))
	}
	if res := history("?limit=2&offset=119"); fmt.Sprint(eventTimestamps(res.Events)) != "[1000 5]" ||
		res.Pagination.Offset != 119 {
		t.Errorf("last page = %v %+v", eventTimestamps(res.Events), res.Pagination)
	}
	if res := history("?actor=nobody&entity_id=https://other.example.org"); res.Pagination.Total != 121 {
		t.Errorf("entity_id/actor must not filter the history route: total %d", res.Pagination.Total)
	}

	resp, body := doRequest(t, app, httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/history?limit=x", id), http.NoBody))
	assertErrorResponse(t, resp, body, http.StatusBadRequest, "invalid_request")

	// A removed subordinate's history is not found on this route.
	resp, body = doRequest(t, app, httptest.NewRequest("DELETE", fmt.Sprintf("/subordinates/%d", other), http.NoBody))
	requireStatus(t, resp, body, http.StatusNoContent)
	resp, body = doRequest(t, app, httptest.NewRequest("GET", fmt.Sprintf("/subordinates/%d/history", other), http.NoBody))
	assertStatus(t, resp, body, http.StatusNotFound)
}

// TestSubordinateHistory_IncludesIDs: history events carry their ID, the
// entity ID and the subordinate ID, like the events query.
func TestSubordinateHistory_IncludesIDs(t *testing.T) {
	t.Parallel()
	app, b := setupSubordinateBaseApp(t)
	id := addSubordinate(t, b.Backends, "https://ids.example.org", model.StatusPending)
	addEvent(t, b.SubordinateEvents, model.SubordinateEvent{SubordinateID: id, Timestamp: 5, Type: model.EventTypeCreated})

	res := getEvents(t, app, fmt.Sprintf("/subordinates/%d/history", id))
	if len(res.Events) != 1 {
		t.Fatalf("got %+v", res)
	}
	ev := res.Events[0]
	if eid, ok := ev["id"].(float64); !ok || eid == 0 {
		t.Errorf("id missing: %+v", ev)
	}
	if ev["subordinate_id"] != float64(id) || ev["entity_id"] != "https://ids.example.org" {
		t.Errorf("ids missing or wrong: %+v", ev)
	}
}
