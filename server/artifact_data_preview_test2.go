package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 大文件只读前 20MB（truncated 由上层结合 TotalRows 判断，这里验证不炸）。
func TestParseLargeTextHead(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.log")
	f, _ := os.Create(p)
	line := strings.Repeat("x", 200) + "\n"
	for i := 0; i < dataPreviewMaxLines*3; i++ {
		_, _ = f.WriteString(line)
	}
	f.Close()
	b, _ := os.ReadFile(p)
	r := parseTextPreview(string(b))
	if !r.Truncated || r.TotalRows != dataPreviewMaxLines*3 {
		t.Fatalf("meta=%+v", r)
	}
}
