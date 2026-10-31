package server

import (
	"context"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// TestChatUnavailableReasonDistinguishesStates pins the operator-facing message
// to the actual configuration state: nothing configured vs. configured-but-not-
// activated must read differently so the user knows what to do.
func TestChatUnavailableReasonDistinguishesStates(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer m.Close()
	td := t.TempDir()
	s := New(context.Background(), m, td, td, td)

	// This test must observe a genuinely empty profile table. That precondition
	// is NOT ours to create: an earlier revision wiped every pre-existing row
	// here and, run against a live database, deleted an operator's real LLM
	// profiles (2026-10-30 incident). Skip instead — CI's fresh DB still runs
	// the test, a shared/live database is left untouched.
	if existing, _ := m.pg.ListProfiles(); len(existing) > 0 {
		t.Skipf("llm_profiles has %d row(s); refusing to wipe a shared database", len(existing))
	}

	if reason := s.chatUnavailableReason(); !strings.Contains(reason, "尚未配置") {
		t.Fatalf("no-profile reason=%q, want 尚未配置", reason)
	}

	id, err := m.pg.SaveProfile(&db.LLMProfile{Name: "p1", Format: "anthropic", Model: "claude-x", APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	// defer (NOT t.Cleanup): t.Cleanup runs after the test's deferred m.Close(),
	// so the original cleanup silently hit "database is closed" and leaked this
	// fixture on every run — on a live install that leak looked like a real
	// (bogus) default profile. A defer registered after m.Close's runs first.
	defer func() {
		if _, err := m.pg.Exec(`UPDATE llm_profiles SET is_default=false WHERE id=$1`, id); err != nil {
			t.Errorf("cleanup deactivate fixture #%d: %v", id, err)
		}
		if err := m.pg.DeleteProfile(id); err != nil {
			t.Errorf("cleanup delete fixture #%d: %v", id, err)
		}
	}()

	// Profile exists but is not activated.
	if reason := s.chatUnavailableReason(); !strings.Contains(reason, "没有已激活") {
		t.Fatalf("inactive reason=%q, want 没有已激活", reason)
	}

	// Once activated, the reason no longer claims a missing/inactive config.
	if err := m.pg.SetActiveProfile(id); err != nil {
		t.Fatal(err)
	}
	if reason := s.chatUnavailableReason(); strings.Contains(reason, "尚未配置") || strings.Contains(reason, "没有已激活") {
		t.Fatalf("active reason=%q should not report missing/inactive", reason)
	}
}

// TestResolveChatAgentHonoursConversationProfile is the regression guard for the
// reported bug: a conversation that picked a valid profile must resolve a chat
// agent even when NO global profile is active, so the send precheck stops
// rejecting it with "LLM 未配置".
func TestResolveChatAgentHonoursConversationProfile(t *testing.T) {
	m, err := NewManager(testDir(t), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer m.Close()
	td := testDir(t)
	s := New(context.Background(), m, td, td, td)

	// Force the global fallback to nil so a non-nil result can ONLY come from the
	// conversation's own profile — this is exactly the situation the user hit
	// (no usable global config) and isolates the fix from the test host's env.
	s.cfgMu.Lock()
	s.chatAgent = nil
	s.cfgMu.Unlock()

	id, err := m.pg.SaveProfile(&db.LLMProfile{Name: "conv-pick", Format: "anthropic", Model: "claude-x", APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	// defer, not t.Cleanup — same close-ordering reason as above.
	defer func() {
		if err := m.pg.DeleteProfile(id); err != nil {
			t.Errorf("cleanup delete fixture #%d: %v", id, err)
		}
	}()

	// A conversation that pinned this profile resolves a chat agent despite the
	// nil global fallback; without the fix the send precheck rejected it outright.
	conv := &db.Conversation{AgentKey: "mainagent", LLMProfileID: &id}
	if s.resolveChatAgent(conv) == nil {
		t.Fatalf("resolveChatAgent(conversation with valid profile) = nil; global fallback wrongly gates the pick")
	}

	// With neither a pick nor a global fallback there is genuinely nothing to run.
	if s.resolveChatAgent(&db.Conversation{AgentKey: "mainagent"}) != nil {
		t.Fatalf("resolveChatAgent with neither pick nor global config should be nil")
	}
}
