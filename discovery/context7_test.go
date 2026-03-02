package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestContext7CanHandle(t *testing.T) {
	p := NewContext7Provider("")
	tests := []struct {
		input string
		want  bool
	}{
		{"react", true},
		{"stripe-node", true},
		{"nextjs", true},
		{"@anthropic-ai/sdk", true},
		{"facebook/react", true},
		{"https://stripe.com", false},
		{"http://example.com", false},
		{"", false},
		{"two words", false},
	}
	for _, tt := range tests {
		if got := p.CanHandle(tt.input); got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestContext7ProviderName(t *testing.T) {
	p := NewContext7Provider("")
	if p.Name() != "context7" {
		t.Errorf("expected name context7, got %s", p.Name())
	}
}

func TestContext7ProviderSatisfiesInterface(t *testing.T) {
	var _ Provider = (*Context7Provider)(nil)
}

func newMockC7Server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/libs/search") {
			resp := struct {
				Results []c7Library `json:"results"`
			}{
				Results: []c7Library{
					{
						ID:            "/facebook/react",
						Title:         "React",
						Description:   "A JavaScript library for building user interfaces",
						TotalSnippets: 150,
						TrustScore:    0.95,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(r.URL.Path, "/context") {
			_, _ = w.Write([]byte("# React Documentation\n\n## Hooks\n\nuseState is a Hook."))
			return
		}
		http.NotFound(w, r)
	}))
}

func TestContext7DiscoverWithMockServer(t *testing.T) {
	srv := newMockC7Server(t)
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "test-key",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	result, err := p.Discover(context.Background(), "react")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if result.Domain != "context7.com~facebook_react" {
		t.Errorf("expected domain context7.com~facebook_react, got %s", result.Domain)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	if result.Files[0].ContentType != TypeContext7 {
		t.Errorf("expected content type %s, got %s", TypeContext7, result.Files[0].ContentType)
	}
	if !strings.Contains(string(result.Files[0].Body), "React Documentation") {
		t.Errorf("expected body to contain React Documentation, got %s", result.Files[0].Body)
	}
}

func TestContext7SearchLibrary(t *testing.T) {
	srv := newMockC7Server(t)
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "test-key",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	libs, err := p.searchLibrary(context.Background(), "react")
	if err != nil {
		t.Fatalf("searchLibrary: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
	if libs[0].ID != "/facebook/react" {
		t.Errorf("expected ID /facebook/react, got %s", libs[0].ID)
	}
	if libs[0].TrustScore != 0.95 {
		t.Errorf("expected trust score 0.95, got %f", libs[0].TrustScore)
	}
}

func TestContext7EmptySearchResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "test-key",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	result, err := p.Discover(context.Background(), "nonexistent-library-xyz")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.Files))
	}
	if result.Domain != "context7.com" {
		t.Errorf("expected domain context7.com, got %s", result.Domain)
	}
}

func TestContext7APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key","message":"Invalid API key"}`))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "bad-key",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	_, err := p.Discover(context.Background(), "react")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestContext7RetryThenSucceed(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/libs/search") {
			resp := struct {
				Results []c7Library `json:"results"`
			}{
				Results: []c7Library{{ID: "/lib/x", Title: "X", TotalSnippets: 10}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_, _ = w.Write([]byte("# X docs\nContent after retry."))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:       "test-key",
		baseURL:      srv.URL,
		client:       srv.Client(),
		retryBackoff: 10 * time.Millisecond,
	}

	result, err := p.Discover(context.Background(), "x")
	if err != nil {
		t.Fatalf("Discover should succeed after retries: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	if !strings.Contains(string(result.Files[0].Body), "Content after retry") {
		t.Error("expected content from successful retry")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestContext7RetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/libs/search") {
			resp := struct {
				Results []c7Library `json:"results"`
			}{
				Results: []c7Library{{ID: "/lib/x", Title: "X", TotalSnippets: 10}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:       "test-key",
		baseURL:      srv.URL,
		client:       srv.Client(),
		retryBackoff: 10 * time.Millisecond,
	}

	_, err := p.Discover(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}
	if !strings.Contains(err.Error(), "202") || !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("expected '202 after 3 attempts' in error, got: %v", err)
	}
}

func TestContext7Retry429(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/libs/search") {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			resp := struct {
				Results []c7Library `json:"results"`
			}{
				Results: []c7Library{{ID: "/lib/x", Title: "X", TotalSnippets: 10}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		_, _ = w.Write([]byte("# docs"))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:       "test-key",
		baseURL:      srv.URL,
		client:       srv.Client(),
		retryBackoff: 10 * time.Millisecond,
	}

	result, err := p.Discover(context.Background(), "x")
	if err != nil {
		t.Fatalf("Discover should succeed after 429 retry: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
}

func TestContext7AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "ctx7sk-test123",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	_, _ = p.searchLibrary(context.Background(), "react")

	if gotAuth != "Bearer ctx7sk-test123" {
		t.Errorf("expected Bearer auth header, got %q", gotAuth)
	}
}

func TestContext7WrappedAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"rate_limited","message":"Too many requests"}`))
	}))
	defer srv.Close()

	p := &Context7Provider{
		apiKey:  "test-key",
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	_, err := p.searchLibrary(context.Background(), "react")
	if err == nil {
		t.Fatal("expected error for wrapped API error")
	}
	if !strings.Contains(err.Error(), "Too many requests") {
		t.Errorf("expected 'Too many requests' in error, got: %v", err)
	}
}

func TestContext7DisplayName(t *testing.T) {
	tests := []struct {
		lib  c7Library
		want string
	}{
		{c7Library{Title: "React", Name: "react"}, "React"},
		{c7Library{Title: "", Name: "react"}, "react"},
		{c7Library{Title: "React"}, "React"},
		{c7Library{Name: "react"}, "react"},
	}
	for _, tt := range tests {
		if got := tt.lib.DisplayName(); got != tt.want {
			t.Errorf("DisplayName() = %q, want %q", got, tt.want)
		}
	}
}
