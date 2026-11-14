package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// v0.4.59：js_audit SPA/webpack 源码泄露审计——只落脚本+组命令，不自己发请求。
func TestJsAuditComposesCommand(t *testing.T) {
	dir := t.TempDir()
	ts := NewToolSet(nil, "worker")
	ts.SetRunDir(dir)
	tool := ts.jsAudit()
	in, _ := json.Marshal(map[string]any{"urls": []string{
		"https://a.example.com/", "https://a.example.com/static/js/app.9f2c1d.js", "bad", "https://a.example.com/",
	}})
	res, err := tool.Call(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var out struct {
		RunWithBash string `json:"run_with_bash"`
		Count       int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, res)), &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, toolResultText(t, res))
	}
	if out.Count != 2 {
		t.Fatalf("count=%d want 2（非法 URL 与重复项应剔除）", out.Count)
	}
	if !strings.Contains(out.RunWithBash, "pojun-jsaudit.py") ||
		!strings.Contains(out.RunWithBash, "--out") ||
		!strings.Contains(out.RunWithBash, "app.9f2c1d.js") {
		t.Fatalf("command=%q", out.RunWithBash)
	}
	// 脚本落盘且是合法 python（py_compile 由 CI 外部验证；这里只查非空）。
	p := filepath.Join(dir, "pojun-jsaudit.py")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("script missing: %v", err)
	}
	if info.Size() < 2000 {
		t.Fatalf("script too small: %d", info.Size())
	}
}

// 没有 runDir 时明确报错，而不是静默成功。
func TestJsAuditRequiresRunDir(t *testing.T) {
	ts := NewToolSet(nil, "reporter")
	tool := ts.jsAudit()
	in, _ := json.Marshal(map[string]any{"urls": []string{"https://a.example.com/"}})
	res, err := tool.Call(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(toolResultText(t, res), "未启用") {
		t.Fatalf("expected 未启用 error, got: %s", toolResultText(t, res))
	}
}
