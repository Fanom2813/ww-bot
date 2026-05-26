package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeProvider is a test double.
type fakeProvider struct {
	name      string
	available bool
	result    string
	err       error
}

func (f *fakeProvider) Name() string                          { return f.name }
func (f *fakeProvider) Available(context.Context) bool         { return f.available }
func (f *fakeProvider) Complete(context.Context, Request) (string, error) {
	return f.result, f.err
}

func TestRegistryFallback(t *testing.T) {
	ctx := context.Background()

	// First is unavailable, second errors, third succeeds.
	reg := NewRegistry(
		&fakeProvider{name: "off", available: false},
		&fakeProvider{name: "broken", available: true, err: errors.New("boom")},
		&fakeProvider{name: "good", available: true, result: "hello"},
	)
	text, who, err := reg.Complete(ctx, Request{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if text != "hello" || who != "good" {
		t.Fatalf("got text=%q who=%q", text, who)
	}

	if avail := reg.Available(ctx); len(avail) != 2 {
		t.Fatalf("want 2 available, got %d", len(avail))
	}
}

func TestRegistryNoProvider(t *testing.T) {
	reg := NewRegistry(&fakeProvider{name: "off", available: false})
	if _, _, err := reg.Complete(context.Background(), Request{}); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("want ErrNoProvider, got %v", err)
	}
}

func TestRegistryAllErrorReturnsFirst(t *testing.T) {
	reg := NewRegistry(
		&fakeProvider{name: "a", available: true, err: errors.New("first")},
		&fakeProvider{name: "b", available: true, err: errors.New("second")},
	)
	if _, _, err := reg.Complete(context.Background(), Request{}); err == nil || err.Error() != "first" {
		t.Fatalf("want first error, got %v", err)
	}
}

func TestOpenAICompatibleSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  hi there  "}}]}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(OpenAIConfig{Name: "test", BaseURL: srv.URL, APIKey: "k", Model: "m", RequiresKey: true})
	if !p.Available(context.Background()) {
		t.Fatal("should be available")
	}
	out, err := p.Complete(context.Background(), Request{System: "sys", Messages: []Message{{Role: User, Content: "yo"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi there" {
		t.Fatalf("want trimmed content, got %q", out)
	}
}

func TestOpenAICompatibleErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(OpenAIConfig{Name: "test", BaseURL: srv.URL, APIKey: "k", Model: "m", RequiresKey: true})
	if _, err := p.Complete(context.Background(), Request{}); err == nil {
		t.Fatal("want error on 401")
	}
}

func TestOpenAIAvailability(t *testing.T) {
	// Hosted provider without key is unavailable.
	hosted := NewOpenAICompatible(OpenAIConfig{Name: "h", BaseURL: "x", Model: "m", RequiresKey: true})
	if hosted.Available(context.Background()) {
		t.Fatal("hosted without key should be unavailable")
	}
	// Local provider needs no key.
	local := NewOpenAICompatible(OpenAIConfig{Name: "ollama", BaseURL: "x", Model: "m", RequiresKey: false})
	if !local.Available(context.Background()) {
		t.Fatal("local should be available without key")
	}
}

func TestCLIAgentStdin(t *testing.T) {
	// "cat" echoes stdin back, simulating a CLI that returns the prompt.
	agent := NewCLIAgent("cat-agent", "cat", nil, true)
	if !agent.Available(context.Background()) {
		t.Skip("cat not on PATH")
	}
	out, err := agent.Complete(context.Background(), Request{Messages: []Message{{Role: User, Content: "ping"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ping") {
		t.Fatalf("want output to contain prompt, got %q", out)
	}
}

func TestCLIAgentUnavailable(t *testing.T) {
	agent := NewCLIAgent("nope", "this-binary-does-not-exist-xyz", nil, true)
	if agent.Available(context.Background()) {
		t.Fatal("nonexistent binary should be unavailable")
	}
}
