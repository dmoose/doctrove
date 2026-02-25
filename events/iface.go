package events

// EventEmitter sends structured events to an observability backend.
// The default implementation posts to an eventrelay HTTP endpoint.
// A no-op emitter is used when no URL is configured. Alternative
// implementations could log to files, send to different backends, etc.
type EventEmitter interface {
	EmitFull(evt Event)
	Flush()
	SetAgentID(id string)
}

// Verify that Emitter satisfies EventEmitter at compile time.
var _ EventEmitter = (*Emitter)(nil)
