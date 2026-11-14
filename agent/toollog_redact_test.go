package agent

import (
	"strings"
	"testing"
)

// v0.4.37 起活动日志的命令摘要保留原文（第一方证据需可溯源，不再脱敏）。
func TestCmdHeadKeepsPlaintext(t *testing.T) {
	cases := []struct {
		in      string
		wantHas string
	}{
		{"sshpass -p 'dolphin!@#' ssh root@10.0.0.1", "dolphin!@#"},
		{"mysql -uroot -p'S3cret!' -h127.0.0.1", "S3cret!"},
		{"curl -H 'Authorization: Bearer abc123XYZ' http://x", "abc123XYZ"},
	}
	for _, c := range cases {
		got := cmdHead(c.in)
		if !strings.Contains(got, c.wantHas) {
			t.Errorf("命令原文被改写: in=%q got=%q want含 %q", c.in, got, c.wantHas)
		}
	}
}

// 摘要只取第一行并限长，避免把多行脚本整段写进活动流。
func TestCmdHeadFirstLineAndTruncate(t *testing.T) {
	got := cmdHead("echo one\necho two")
	if strings.Contains(got, "echo two") || !strings.Contains(got, "echo one") {
		t.Fatalf("cmdHead must keep only first line, got %q", got)
	}
	long := strings.Repeat("A", 200)
	got = cmdHead(long)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > 97 {
		t.Fatalf("cmdHead must truncate with ellipsis, got len=%d", len([]rune(got)))
	}
}
