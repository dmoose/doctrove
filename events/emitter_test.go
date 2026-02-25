package events

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestNoOpEmitter(t *testing.T) {
	e := New("", "test")
	// Should not panic
	e.Emit("action", "agent1", map[string]any{"key": "value"})
	e.Flush()
}

func TestEmitterSendsEvents(t *testing.T) {
	var mu sync.Mutex
	var received []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			t.Errorf("decode error: %v", err)
			return
		}
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := New(srv.URL, "test-source")
	e.Emit("init", "agent-1", map[string]any{"domain": "example.com"})
	e.Emit("sync", "", map[string]any{"domain": "example.com", "added": 5})
	e.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	// Events are async — order is not guaranteed, so check by action
	actions := map[string]Event{}
	for _, evt := range received {
		actions[evt.Action] = evt
	}

	initEvt, ok := actions["init"]
	if !ok {
		t.Fatal("missing init event")
	}
	if initEvt.Source != "test-source" {
		t.Errorf("expected source test-source, got %s", initEvt.Source)
	}
	if initEvt.AgentID != "agent-1" {
		t.Errorf("expected agent_id agent-1, got %s", initEvt.AgentID)
	}
	if _, ok := actions["sync"]; !ok {
		t.Error("missing sync event")
	}
}

func TestEmitterHandlesServerDown(t *testing.T) {
	// Point to a URL that won't respond
	e := New("http://127.0.0.1:1", "test-source")
	e.Emit("test", "", nil)
	e.Flush() // Should not hang or panic
}

func TestEmitterEventTimestamp(t *testing.T) {
	var mu sync.Mutex
	var received []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		_ = json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := New(srv.URL, "ts-test")
	e.Emit("ping", "", nil)
	e.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].TS.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestEmitterDataPassthrough(t *testing.T) {
	var mu sync.Mutex
	var received []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		_ = json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := New(srv.URL, "data-test")
	e.Emit("sync", "agent-x", map[string]any{
		"domain": "example.com",
		"added":  float64(3),
	})
	e.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	evt := received[0]
	if evt.Data["domain"] != "example.com" {
		t.Errorf("expected domain example.com, got %v", evt.Data["domain"])
	}
	if evt.AgentID != "agent-x" {
		t.Errorf("expected agent_id agent-x, got %s", evt.AgentID)
	}
}

func TestEmitterNilData(t *testing.T) {
	var mu sync.Mutex
	var received []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		_ = json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := New(srv.URL, "nil-data")
	e.Emit("test", "", nil)
	e.Flush()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Action != "test" {
		t.Errorf("expected action test, got %s", received[0].Action)
	}
}
