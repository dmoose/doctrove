package discovery

import (
	"context"
	"testing"
	"time"
)

// stubProvider is a test provider that returns canned results.
type stubProvider struct {
	name      string
	canHandle bool
	result    *Result
	err       error
}

func (s *stubProvider) Name() string                                          { return s.name }
func (s *stubProvider) CanHandle(input string) bool                           { return s.canHandle }
func (s *stubProvider) Discover(ctx context.Context, input string) (*Result, error) {
	return s.result, s.err
}

func TestProviderInterface(t *testing.T) {
	// Verify stubProvider satisfies Provider
	var _ Provider = (*stubProvider)(nil)
	var _ Provider = (*SiteProvider)(nil)
}

func TestDiscovererRouting(t *testing.T) {
	resultA := &Result{Domain: "a.com", DiscoveredAt: time.Now()}
	resultB := &Result{Domain: "b.com", DiscoveredAt: time.Now()}

	d := NewWithProviders(
		&stubProvider{name: "first", canHandle: false, result: resultA},
		&stubProvider{name: "second", canHandle: true, result: resultB},
	)

	got, err := d.Discover(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Domain != "b.com" {
		t.Errorf("expected domain b.com, got %s", got.Domain)
	}
}

func TestDiscovererNoMatch(t *testing.T) {
	d := NewWithProviders(
		&stubProvider{name: "nope", canHandle: false},
	)

	got, err := d.Discover(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Files) != 0 {
		t.Errorf("expected empty result, got %d files", len(got.Files))
	}
}

func TestSiteProviderCanHandle(t *testing.T) {
	p := &SiteProvider{}
	tests := []struct {
		input string
		want  bool
	}{
		{"https://stripe.com", true},
		{"http://example.com", true},
		{"react", false},
		{"stripe@latest", false},
		{"", false},
		{"ftp://x", false},
	}
	for _, tt := range tests {
		if got := p.CanHandle(tt.input); got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRegisterProvider(t *testing.T) {
	d := New(nil, nil, 0)
	if len(d.Providers()) != 1 {
		t.Fatalf("expected 1 default provider, got %d", len(d.Providers()))
	}

	d.RegisterProvider(&stubProvider{name: "extra"})
	if len(d.Providers()) != 2 {
		t.Errorf("expected 2 providers after register, got %d", len(d.Providers()))
	}
}

func TestDiscoveredFileBody(t *testing.T) {
	f := DiscoveredFile{
		Path:        "/docs/api.md",
		ContentType: TypeCompanion,
		Body:        []byte("# API Docs\nSome content here"),
		Size:        30,
	}
	if f.Body == nil {
		t.Error("expected Body to be set")
	}
}
