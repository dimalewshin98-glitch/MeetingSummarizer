package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type compressWriter struct {
	w         http.ResponseWriter
	zw        *gzip.Writer
	isGzipped bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:         w,
		zw:        gzip.NewWriter(w),
		isGzipped: false,
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if c.isGzipped {
		return c.zw.Write(p)
	} else {
		return c.w.Write(p)
	}
}

func (c *compressWriter) WriteHeader(statusCode int) {
	contentType := c.w.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/html") {
		c.w.Header().Set("Content-Encoding", "gzip")
		c.isGzipped = true
	}
	c.w.WriteHeader(statusCode)
}

func (c *compressWriter) Close() error {
	if c.isGzipped {
		return c.zw.Close()
	}
	return nil
}

type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

func GzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			cw := newCompressWriter(w)
			ow = cw
			defer cw.Close()
		}
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				http.Error(ow, "Error uncompressing request", http.StatusBadRequest)
				return
			}
			r.Body = cr
			defer cr.Close()
		}
		h.ServeHTTP(ow, r)
	})
}
