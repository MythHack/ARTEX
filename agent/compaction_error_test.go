package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

func TestCompactionEmptyBodyReportsStopReason(t *testing.T) {
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
	for _, detail := range []string{"empty body from model", `stop_reason="max_tokens"`, "output_tokens=1500", "requested_max_tokens=1500"} {
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("diagnostic %q is missing %q", err, detail)
		}
	}
	if strings.Contains(err.Error(), "fixture reasoning") {
		t.Fatal("diagnostic must not include model content")
	}
}
