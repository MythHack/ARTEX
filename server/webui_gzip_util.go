package server

import (
	"bytes"
	"compress/gzip"
)

// gzipBuffer 压缩 data；失败返回 nil。BestSpeed——内容寻址资源只压一次，
// CPU 不敏感，压得快不如常驻省心。
func gzipBuffer(data []byte) []byte {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil
	}
	if _, err := w.Write(data); err != nil {
		return nil
	}
	if err := w.Close(); err != nil {
		return nil
	}
	return buf.Bytes()
}
