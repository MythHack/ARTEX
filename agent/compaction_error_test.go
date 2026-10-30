package agent

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/norma/llm"
	_ "modernc.org/sqlite"
)

func TestCompactionRejectsReasoningOnlyTruncation(t *testing.T) {
	const response = `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"fixture reasoning"}]}],"usage":{"input_tokens":867,"output_tokens":1500}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	defer srv.Close()

	prov, err := (Config{
		Format:  llm.FormatOpenAIResponses,
		BaseURL: srv.URL,
		APIKey:  "fixture-only",
		Model:   "fixture-model",
	}).NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompactor(prov, "fixture-model")
	body, err := c.compress(context.Background(), &coldGraph{}, block{}, nil)
	if err == nil || body != "" {
		t.Fatalf("body=%q err=%v; expected an empty-body diagnostic", body, err)
	}
	for _, detail := range []string{"incomplete model response", `stop_reason="max_tokens"`, "output_tokens=1500", "requested_max_tokens=1500"} {
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("diagnostic %q is missing %q", err, detail)
		}
	}
	if strings.Contains(err.Error(), "fixture reasoning") {
		t.Fatal("diagnostic must not include model content")
	}
}

func TestCompactionRejectsTruncatedText(t *testing.T) {
	const response = `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"The meeting starts at"}]}],"usage":{"output_tokens":1500}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	defer srv.Close()
	prov, err := (Config{Format: llm.FormatOpenAIResponses, BaseURL: srv.URL, APIKey: "fixture", Model: "fixture"}).NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	body, err := NewCompactor(prov, "fixture").compress(t.Context(), &coldGraph{}, block{}, nil)
	if err == nil || body != "" {
		t.Fatalf("truncated text accepted: body=%q err=%v", body, err)
	}
	if !strings.Contains(err.Error(), `stop_reason="max_tokens"`) {
		t.Fatalf("missing truncation reason: %v", err)
	}
}

type compactionCancelProvider struct {
	cancel context.CancelFunc
	calls  int
	late   bool
}

func (p *compactionCancelProvider) Complete(ctx context.Context, _ llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.calls++
	p.cancel()
	if p.late {
		return llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock("Meeting at noon.")}}, "end_turn", llm.Usage{}, nil
	}
	return llm.Message{}, "", llm.Usage{}, ctx.Err()
}

func (p *compactionCancelProvider) Stream(context.Context, llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	panic("unexpected streaming call")
}

// A disposable SQLite store exercises the batch loops and persistence boundary
// without loading application configuration or connecting to PostgreSQL.
func compactionBatchFixture(t *testing.T) (*db.DB, *db.ExplorationStore) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	_, err = sqlDB.Exec(`
CREATE TABLE explorations (id INTEGER PRIMARY KEY, round_no INTEGER);
INSERT INTO explorations VALUES (1, 10);
CREATE TABLE exploration_nodes (
 id INTEGER PRIMARY KEY, exploration_id INTEGER, kind TEXT, payload TEXT,
 priority INTEGER DEFAULT 0, state TEXT, origin TEXT, owner TEXT,
 blocked_reason TEXT, delete_reason TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 cold_since_round INTEGER, content_version INTEGER DEFAULT 0
);
CREATE TABLE exploration_edges (
 exploration_id INTEGER, src_id INTEGER, rel TEXT, dst_id INTEGER,
 UNIQUE (exploration_id, src_id, rel, dst_id)
);
INSERT INTO exploration_nodes (id, exploration_id, kind, payload, state, cold_since_round) VALUES
 (1,1,'fact','{"summary":"Meeting at noon."}','done',0),
 (2,1,'fact','{"summary":"Meeting room A."}','done',0),
 (3,1,'fact','{"summary":"Lunch at one."}','done',0),
 (4,1,'fact','{"summary":"Lunch in the cafeteria."}','done',0);
INSERT INTO exploration_edges VALUES (1,1,'derived_from',2), (1,3,'derived_from',4);
`)
	if err != nil {
		t.Fatal(err)
	}
	d := &db.DB{DB: sqlDB}
	return d, d.Exploration(1)
}

func TestCompactionBatchStopsOnCancellation(t *testing.T) {
	for _, mode := range []string{"minor", "major"} {
		for _, late := range []bool{false, true} {
			name := mode + "/error"
			if late {
				name = mode + "/late_success"
			}
			t.Run(name, func(t *testing.T) {
				d, es := compactionBatchFixture(t)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				prov := &compactionCancelProvider{cancel: cancel, late: late}
				c := NewCompactor(prov, "fixture")
				if mode == "minor" {
					c.minor(ctx, es)
				} else {
					c.major(ctx, es)
				}
				if prov.calls != 1 {
					t.Errorf("model calls=%d, want one call before cancellation", prov.calls)
				}
				var digests int
				if err := d.QueryRow("SELECT COUNT(*) FROM exploration_nodes WHERE kind='digest'").Scan(&digests); err != nil {
					t.Fatal(err)
				}
				if digests != 0 {
					t.Errorf("persisted %d digests after cancellation", digests)
				}
			})
		}
	}
}

func TestCompactionBatchPersistsCompletedSummaries(t *testing.T) {
	for _, mode := range []string{"minor", "major"} {
		t.Run(mode, func(t *testing.T) {
			d, es := compactionBatchFixture(t)
			prov := &compactionCancelProvider{cancel: func() {}, late: true}
			c := NewCompactor(prov, "fixture")
			if mode == "minor" {
				c.minor(t.Context(), es)
			} else {
				c.major(t.Context(), es)
			}
			var digests int
			if err := d.QueryRow("SELECT COUNT(*) FROM exploration_nodes WHERE kind='digest' AND state='active'").Scan(&digests); err != nil {
				t.Fatal(err)
			}
			if prov.calls != 2 || digests != 2 {
				t.Fatalf("calls=%d digests=%d, want two completed summaries", prov.calls, digests)
			}
		})
	}
}

func TestCompactionDoesNotCallProviderAfterCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "canceled"
		if deadline {
			name = "deadline_exceeded"
		}
		t.Run(name, func(t *testing.T) {
			var ctx context.Context
			var cancel context.CancelFunc
			wantErr := context.Canceled
			if deadline {
				ctx, cancel = context.WithDeadline(t.Context(), time.Time{})
				wantErr = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(t.Context())
				cancel()
			}
			defer cancel()
			prov := &compactionCancelProvider{cancel: cancel, late: true}
			body, err := NewCompactor(prov, "fixture").compress(ctx, &coldGraph{}, block{}, nil)
			if prov.calls != 0 || body != "" || !errors.Is(err, wantErr) {
				t.Fatalf("calls=%d body=%q err=%v; want no call and %v", prov.calls, body, err, wantErr)
			}
		})
	}
}
