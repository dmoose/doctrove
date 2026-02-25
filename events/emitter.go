package events

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Event is the structured payload sent to the event relay.
type Event struct {
	Source     string         `json:"source"`
	Channel   string         `json:"channel,omitempty"`
	Action    string         `json:"action"`
	Level     string         `json:"level,omitempty"`
	AgentID   string         `json:"agent_id,omitempty"`
	DurationMS int64         `json:"duration_ms"`
	Data      map[string]any `json:"data,omitempty"`
	TS        time.Time      `json:"ts"`
}

// Emitter sends events to an event relay service.
// If URL is empty, all operations are no-ops (zero overhead).
type Emitter struct {
	url     string
	source  string
	agentID string
	client  *http.Client
	wg      sync.WaitGroup
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

// SetAgentID sets the default agent ID used when callers pass an empty string.
func (e *Emitter) SetAgentID(id string) {
	e.agentID = id
}

// Emit sends an event to the relay. Fire-and-forget — never blocks, never errors.
// If agentID is empty, the emitter's default agent ID is used.
func (e *Emitter) Emit(action, agentID string, data map[string]any) {
	e.EmitFull(Event{Action: action, AgentID: agentID, Data: data})
}

// EmitFull sends a fully specified event to the relay.
// Source, TS, and AgentID (when empty) are filled automatically.
func (e *Emitter) EmitFull(evt Event) {
	if e.url == "" {
		return
	}
	evt.Source = e.source
	evt.TS = time.Now()
	if evt.AgentID == "" {
		evt.AgentID = e.agentID
	}
	e.wg.Go(func() {
		e.send(evt)
	})
}

// Flush waits for all pending events to be sent.
func (e *Emitter) Flush() {
	e.wg.Wait()
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
	_ = resp.Body.Close()
}
