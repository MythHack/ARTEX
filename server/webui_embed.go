//go:build embedui

package server

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// webuiDist holds the statically-exported frontend (built with
// `cd web && npm run build:static`, then copied into server/webui/dist). Compiled
// into the binary only when building with `-tags embedui`. The `all:` prefix is
// required so Next's `_next/` asset dir (leading underscore) is included.
//
//go:embed all:webui/dist
var webuiDist embed.FS

// webuiHandler serves the embedded SPA. Public (no JWT) — auth is enforced
// client-side and on /api. Serves the exported per-route index.html files and
// falls back to index.html so client-side routing still resolves unknown paths.
func (s *Server) webuiHandler() http.Handler {
	root, err := fs.Sub(webuiDist, "webui/dist")
	if err != nil {
		panic(err)
	}
	// RuoJi6 #158：启动时把内容寻址静态资源预压缩进内存（约省 70% 首屏体积）。
	webuiGzip.compressAll(root, "webui")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		// 缓存策略（v0.3.99）：embed FS 没有 Last-Modified，浏览器对无缓存头的
		// 响应走启发式缓存——每次一键更新后，老用户的浏览器可能仍用旧 HTML/JS
		// 渲染旧界面（实测：新功能上线后用户「看不见」）。因此：
		//   - 内容寻址的 _next/static/*（文件名带 hash）→ 一年 immutable；
		//   - 其余（HTML 入口、图标等）→ no-cache，每次回源校验拿最新。
		if strings.Contains(p, "_next/static/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		// 1) exact file (assets: _next/*, favicon.ico, ...)
		// 2) route dir → <p>/index.html (trailingSlash export)
		// 3) <p>.html
		// 4) SPA 兜底 → index.html（交给客户端路由）
		for _, cand := range []string{p, p + "/index.html", p + ".html"} {
			if serveFileIfExists(w, r, root, cand) {
				return
			}
		}
		http.ServeFileFS(w, r, root, "index.html")
	})
}

// serveFileIfExists serves name from fsys when it exists as a regular file.
func serveFileIfExists(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	st, statErr := f.Stat()
	_ = f.Close()
	if statErr != nil || st.IsDir() {
		return false
	}
	// #158：命中预压缩副本且客户端接受 gzip 时直接回内存字节。
	if gz := webuiGzip.get("webui", name); gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		ctype := mime.TypeByExtension(path.Ext(name))
		if ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gz)
		return true
	}
	http.ServeFileFS(w, r, fsys, name)
	return true
}
