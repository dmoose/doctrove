package events

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// Event is the structured payload sent to the event relay.
type Event struct {
	Source  string         `json:"source"`
	Action  string         `json:"action"`
	AgentID string         `json:"agent_id,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	TS      time.Time      `json:"ts"`
}

// Emitter sends events to an event relay service.
// If URL is empty, all operations are no-ops (zero overhead).
type Emitter struct {
	url    string
	source string
	client *http.Client
}

// New creates an Emitter. If url is empty, returns a no-op emitter.
func New(url, source string) *Emitter {
	if url == "" {
		return &Emitter{}
	}
	return &Emitter{
		url:    url,
		source: source,
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// Emit sends an event to the relay. Fire-and-forget — never blocks, never errors.
func (e *Emitter) Emit(action, agentID string, data map[string]any) {
	if e.url == "" {
		return
	}
	evt := Event{
		Source:  e.source,
		Action:  action,
		AgentID: agentID,
		Data:    data,
		TS:      time.Now(),
	}
	go e.send(evt)
}

func (e *Emitter) send(evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		return
	}
	resp, err := e.client.Post(e.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}
