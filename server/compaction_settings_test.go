package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/agent"
)

// TestCompactionBudgetSettingsRoundTrip covers the settings API surface for the
// built-in compaction body budget: defaults match the agent package, invalid
// values are rejected, and a PUT is reflected by both the Manager and GET
// /api/settings — the resolver chain the Compactor reads on every compress.
func TestCompactionBudgetSettingsRoundTrip(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()

	if got := m.CompactionMaxTokens(); got != agent.DefaultCompactionMaxTokens {
		t.Fatalf("default max tokens=%d, want %d", got, agent.DefaultCompactionMaxTokens)
	}
	if got := m.CompactionThinking(); got != agent.DefaultCompactionThinking {
		t.Fatalf("default thinking=%q, want %q", got, agent.DefaultCompactionThinking)
	}
	if err := m.SetCompactionMaxTokens(100); err == nil {
		t.Fatal("too-small cap accepted")
	}
	if err := m.SetCompactionThinking("maybe"); err == nil {
		t.Fatal("unknown thinking mode accepted")
	}

	oldTokens, tokensSet, _ := m.pg.GetSetting(settingCompactionMaxTokens)
	oldThinking, thinkingSet, _ := m.pg.GetSetting(settingCompactionThinking)
	defer func() {
		if tokensSet {
			_ = m.pg.SetSetting(settingCompactionMaxTokens, oldTokens)
		} else {
			_, _ = m.pg.Exec("DELETE FROM settings WHERE key=$1", settingCompactionMaxTokens)
		}
		if thinkingSet {
			_ = m.pg.SetSetting(settingCompactionThinking, oldThinking)
		} else {
			_, _ = m.pg.Exec("DELETE FROM settings WHERE key=$1", settingCompactionThinking)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, m, t.TempDir(), t.TempDir(), t.TempDir())

	put := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"compaction_max_tokens":8000,"compaction_thinking":"enabled"}`))
	putResult := httptest.NewRecorder()
	s.putSettings(putResult, put)
	if putResult.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", putResult.Code, putResult.Body.String())
	}
	if m.CompactionMaxTokens() != 8000 || m.CompactionThinking() != "enabled" {
		t.Fatalf("PUT did not take: tokens=%d thinking=%q", m.CompactionMaxTokens(), m.CompactionThinking())
	}

	res := httptest.NewRecorder()
	s.getSettings(res, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if n, _ := payload["compaction_max_tokens"].(float64); int(n) != 8000 {
		t.Fatalf("GET compaction_max_tokens=%v, want 8000", payload["compaction_max_tokens"])
	}
	if v, _ := payload["compaction_thinking"].(string); v != "enabled" {
		t.Fatalf("GET compaction_thinking=%v, want enabled", payload["compaction_thinking"])
	}

	bad := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"compaction_max_tokens":10}`))
	badResult := httptest.NewRecorder()
	s.putSettings(badResult, bad)
	if badResult.Code != http.StatusBadRequest {
		t.Fatalf("invalid PUT status=%d body=%s", badResult.Code, badResult.Body.String())
	}
	if m.CompactionMaxTokens() != 8000 {
		t.Fatalf("invalid PUT changed the stored cap: %d", m.CompactionMaxTokens())
	}
}

// TestCompactionBudgetSettingsKeysStable locks the persisted key names and the
// default cap — the settings table is read by name across restarts, and 6000 is
// the documented default surfaced in the UI.
func TestCompactionBudgetSettingsKeysStable(t *testing.T) {
	if settingCompactionMaxTokens != "compaction_max_tokens" || settingCompactionThinking != "compaction_thinking" {
		t.Fatalf("keys drifted: %q %q", settingCompactionMaxTokens, settingCompactionThinking)
	}
	if agent.DefaultCompactionMaxTokens != 6000 {
		t.Fatalf("default cap changed: %d", agent.DefaultCompactionMaxTokens)
	}
}
