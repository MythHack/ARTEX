package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

// TestCompactionBodyBudgetDefaultsAndOverride pins the settings-backed §4 call
// budget: the default cap is large enough for a reasoning model, per-call
// thinking=disabled suppresses the profile reasoning effort, injected resolvers
// win, and non-positive/empty resolver values fall back to the defaults.
// Everything is asserted on the wire body the provider actually sent.
func TestCompactionBodyBudgetDefaultsAndOverride(t *testing.T) {
	const complete = `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"digest body"}]}],"usage":{"input_tokens":10,"output_tokens":20}}`
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, complete)
	}))
	defer srv.Close()

	prov, err := (Config{
		Format:          llm.FormatOpenAIResponses,
		BaseURL:         srv.URL,
		APIKey:          "fixture",
		Model:           "fixture",
		ReasoningEffort: "low", // profile-level effort; per-call "disabled" must suppress it
	}).NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompactor(prov, "fixture")
	last := func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return bodies[len(bodies)-1]
	}

	// Default budget: DefaultCompactionMaxTokens, thinking disabled → no reasoning field.
	body, err := c.compress(context.Background(), &coldGraph{}, block{}, nil)
	if err != nil || body != "digest body" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	got := last()
	if n, _ := got["max_output_tokens"].(float64); int(n) != DefaultCompactionMaxTokens {
		t.Fatalf("default max_output_tokens=%v, want %d", got["max_output_tokens"], DefaultCompactionMaxTokens)
	}
	if _, has := got["reasoning"]; has {
		t.Fatalf("default thinking=disabled must suppress reasoning: %v", got["reasoning"])
	}

	// Injected budget wins and thinking=enabled lets the profile effort through.
	c.SetBodyBudget(func() int { return 1234 }, func() string { return "enabled" })
	if _, err := c.compress(context.Background(), &coldGraph{}, block{}, nil); err != nil {
		t.Fatal(err)
	}
	got = last()
	if n, _ := got["max_output_tokens"].(float64); int(n) != 1234 {
		t.Fatalf("injected max_output_tokens=%v, want 1234", got["max_output_tokens"])
	}
	reasoning, ok := got["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "low" {
		t.Fatalf("thinking=enabled must pass the profile reasoning effort: %v", got["reasoning"])
	}

	// Non-positive/empty resolver values fall back to the defaults.
	c.SetBodyBudget(func() int { return 0 }, func() string { return "  " })
	if _, err := c.compress(context.Background(), &coldGraph{}, block{}, nil); err != nil {
		t.Fatal(err)
	}
	got = last()
	if n, _ := got["max_output_tokens"].(float64); int(n) != DefaultCompactionMaxTokens {
		t.Fatalf("fallback max_output_tokens=%v, want %d", got["max_output_tokens"], DefaultCompactionMaxTokens)
	}
	if _, has := got["reasoning"]; has {
		t.Fatalf("fallback thinking must be disabled: %v", got["reasoning"])
	}
}
