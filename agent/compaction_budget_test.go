package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
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
		ReasoningEffort: "low",
	}).NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompactor(prov, "fixture")
	c.SetBodyBudget(func() int { return DefaultCompactionMaxTokens }, func() string { return DefaultCompactionThinking })
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

type fakeProvider struct {
	mu    sync.Mutex
	calls []fakeCall
	reply func(call int, maxTokens int, input string) string
	stop  func(call int) string
}

type fakeCall struct {
	MaxTokens int
	Input     string
}

func (f *fakeProvider) Stream(ctx context.Context, req llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) {}
}

func (f *fakeProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	in := ""
	for _, m := range req.Messages {
		in += m.Text()
	}
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{MaxTokens: req.MaxTokens, Input: in})
	idx := len(f.calls) - 1
	f.mu.Unlock()
	body := ""
	if f.reply != nil {
		body = f.reply(idx, req.MaxTokens, in)
	}
	stop := ""
	if f.stop != nil {
		stop = f.stop(idx)
	}
	msg := llm.Message{}
	if body != "" {
		msg.Content = []llm.ContentBlock{llm.TextBlock(body)}
	}
	return msg, stop, llm.Usage{InputTokens: 100, OutputTokens: 50}, nil
}

func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeProvider) snapshotCalls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func TestCompressOutputBudgetIsGenerous(t *testing.T) {
	prov := &fakeProvider{reply: func(int, int, string) string { return "摘要正文" }}
	c := NewCompactor(prov, "test-model")

	if c.outTokens < 32768 {
		t.Errorf("默认输出预算太小: %d（思考型模型需要足够冗余）", c.outTokens)
	}
	if c.maxDur < 10*time.Minute {
		t.Errorf("默认墙钟上限太小: %v（长输入的思考型模型要跑很久）", c.maxDur)
	}
	if c.maxBlockMembers != 6 {
		t.Errorf("默认单块上限 %d，应为 6（实测最稳）", c.maxBlockMembers)
	}
}

func TestCompressUsesConfiguredOutputBudget(t *testing.T) {
	prov := &fakeProvider{reply: func(int, int, string) string { return "摘要正文" }}
	c := NewCompactor(prov, "test-model")
	c.SetOutputTokens(65536)

	g := &coldGraph{}
	b := block{Members: []int64{1, 2}}
	body, err := c.compress(context.Background(), g, b, map[int64]*db.Node{})
	if err != nil {
		t.Fatalf("compress 失败: %v", err)
	}
	if body != "摘要正文" {
		t.Errorf("正文不对: %q", body)
	}
	if prov.callCount() != 1 {
		t.Fatalf("应调用一次,实际 %d 次", prov.callCount())
	}
	if prov.snapshotCalls()[0].MaxTokens != 65536 {
		t.Errorf("未使用配置的输出预算: %d", prov.snapshotCalls()[0].MaxTokens)
	}
}

func TestCompactorSettersFallBackToDefaults(t *testing.T) {
	c := NewCompactor(&fakeProvider{}, "m")
	c.SetOutputTokens(1234)
	c.SetMaxDuration(3 * time.Minute)
	if c.outTokens != 1234 {
		t.Errorf("SetOutputTokens 未生效: %d", c.outTokens)
	}
	if c.maxDur != 3*time.Minute {
		t.Errorf("SetMaxDuration 未生效: %v", c.maxDur)
	}
	c.SetOutputTokens(0)
	c.SetMaxDuration(0)
	if c.outTokens != defaultCompactionTokens {
		t.Errorf("0 应恢复默认预算,得到 %d", c.outTokens)
	}
	if c.maxDur != defaultCompactionDuration {
		t.Errorf("0 应恢复默认时长,得到 %v", c.maxDur)
	}
}

func TestCompressEmptyBodyCarriesDiagnostics(t *testing.T) {
	prov := &fakeProvider{
		reply: func(int, int, string) string { return "" },
		stop:  func(int) string { return "length" },
	}
	c := NewCompactor(prov, "test-model")
	g := &coldGraph{}
	b := block{Members: []int64{1}}
	_, err := c.compress(context.Background(), g, b, map[int64]*db.Node{})
	if err == nil {
		t.Fatal("空正文应报错")
	}
	msg := err.Error()
	for _, want := range []string{"empty body", "length", "in=", "out="} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误信息缺少 %q: %s", want, msg)
		}
	}
}

func TestCompressSplitsOversizedBlock(t *testing.T) {
	var prov fakeProvider
	prov.reply = func(call int, maxTokens int, input string) string {
		return fmt.Sprintf("子块%d", call)
	}
	c := NewCompactor(&prov, "test-model")
	c.SetMaxBlockMembers(4)

	members := make([]int64, 10)
	nodeByID := map[int64]*db.Node{}
	for i := range members {
		members[i] = int64(i + 1)
		nodeByID[members[i]] = &db.Node{ID: members[i], Kind: db.KindFact, State: "confirmed"}
	}
	g := &coldGraph{}
	b := block{Members: members}
	bodies, err := c.compressChunked(context.Background(), g, b, nodeByID)
	if err != nil {
		t.Fatalf("compressChunked 失败: %v", err)
	}
	if len(bodies) != 3 {
		t.Errorf("10 个成员按 4 切应得 3 块,得到 %d", len(bodies))
	}
	if prov.callCount() != 3 {
		t.Errorf("应发起 3 次调用,实际 %d", prov.callCount())
	}
	for i, call := range prov.snapshotCalls() {
		n := strings.Count(call.Input, "\n- #")
		if n > 4 {
			t.Errorf("第 %d 次调用含 %d 个成员,超过阈值 4", i, n)
		}
	}
}

func TestCompressThresholdZeroRestoresDefault(t *testing.T) {
	prov := &fakeProvider{reply: func(int, int, string) string { return "x" }}
	c := NewCompactor(prov, "m")
	c.SetMaxBlockMembers(4)
	c.SetMaxBlockMembers(0)
	if c.maxBlockMembers != defaultCompactionMaxMembers {
		t.Errorf("0 应恢复默认阈值,得到 %d", c.maxBlockMembers)
	}
}

func TestBuildCompressionInputEscapesNodeText(t *testing.T) {
	nodeByID := map[int64]*db.Node{
		1: {ID: 1, Kind: db.KindFact, State: "confirmed", Payload: []byte(`{"summary":"含\"引号\"和\n换行"}`)},
	}
	g := &coldGraph{}
	in := buildCompressionInput(g, block{Members: []int64{1}}, nodeByID)
	if !strings.Contains(in, `含"引号"`) {
		t.Errorf("节点摘要未带出: %s", in)
	}
}

func TestCompressChunkedRunsSubBlocksConcurrently(t *testing.T) {
	const (
		members  = 40
		limit    = 4
		wantSubs = members / limit
	)
	var (
		mu      sync.Mutex
		inFlt   = 0
		maxSeen = 0
	)
	prov := &fakeProvider{}
	prov.reply = func(call int, maxTokens int, input string) string {
		mu.Lock()
		inFlt++
		if inFlt > maxSeen {
			maxSeen = inFlt
		}
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		inFlt--
		mu.Unlock()
		return fmt.Sprintf("子块%d", call)
	}

	c := NewCompactor(prov, "test-model")
	c.SetMaxBlockMembers(limit)

	ms := make([]int64, members)
	nodeByID := map[int64]*db.Node{}
	for i := range ms {
		ms[i] = int64(i + 1)
		nodeByID[ms[i]] = &db.Node{ID: ms[i], Kind: db.KindFact, State: "confirmed"}
	}
	bodies, err := c.compressChunked(context.Background(), &coldGraph{}, block{Members: ms}, nodeByID)
	if err != nil {
		t.Fatalf("compressChunked 失败: %v", err)
	}
	if len(bodies) != wantSubs {
		t.Fatalf("应得 %d 个子块摘要,得到 %d", wantSubs, len(bodies))
	}
	if prov.callCount() != wantSubs {
		t.Errorf("应发起 %d 次调用,实际 %d", wantSubs, prov.callCount())
	}
	mu.Lock()
	observed := maxSeen
	mu.Unlock()
	if observed < 2 {
		t.Errorf("子块应并发执行,但同时在飞的最多只有 %d 个（并发未生效）", observed)
	}
	if observed > compactionSubConcurrency {
		t.Errorf("同时在飞的子块 %d 个,超过并发上限 %d", observed, compactionSubConcurrency)
	}
}
