package server

import (
	"io/fs"
	"strings"
	"sync"
)

// gzipCache 预压缩 embed 静态资源（RuoJi6 #158）：首屏 2.6MB 未压缩在直连
// :8787 时是主要瓶颈。_next/static/* 内容寻址不可变——启动时一次性 gzip 到
// 内存（best-speed 级别即可，静态资源只压一次），请求带 Accept-Encoding: gzip
// 时直接回内存副本并设置 Content-Encoding: gzip + immutable。
type gzipEntry struct {
	data    []byte
	serving bool // gzip 中（false = 全部完成）
}

type gzipFileCache struct {
	mu    sync.RWMutex
	byFSS map[string]map[string][]byte // fsys 标识 → path → gzip 字节
}

var webuiGzip = &gzipFileCache{}

// compressAll 预压缩 fsys 内全部 _next/static/ 资源。幂等；启动时调用一次。
func (g *gzipFileCache) compressAll(fsys fs.FS, key string) {
	g.mu.Lock()
	if g.byFSS == nil {
		g.byFSS = map[string]map[string][]byte{}
	}
	if _, ok := g.byFSS[key]; ok {
		g.mu.Unlock()
		return
	}
	// 占位防并发重复压缩
	g.byFSS[key] = map[string][]byte{}
	g.mu.Unlock()

	out := map[string][]byte{}
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.Contains(p, "_next/static/") {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil
		}
		// gzip.BestSpeed：只压一次、常驻内存，速度优先。
		buf := gzipBuffer(data)
		if buf == nil {
			return nil
		}
		out[p] = buf
		return nil
	})

	g.mu.Lock()
	g.byFSS[key] = out
	g.mu.Unlock()
}

// get 返回预压缩副本（nil = 未命中）。
func (g *gzipFileCache) get(key, p string) []byte {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if m, ok := g.byFSS[key]; ok {
		return m[p]
	}
	return nil
}
