package logger

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

var Log *slog.Logger

func Initialize(level string) error {
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return err
	}
	var handler slog.Handler
	handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	Log = slog.New(handler)
	return nil
}

type loggedResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int
}

func (rw *loggedResponseWriter) WriteHeader(code int) {
	rw.ResponseWriter.WriteHeader(code)
	rw.statusCode = code
}

func (lw *loggedResponseWriter) Write(data []byte) (int, error) {
	size, err := lw.ResponseWriter.Write(data)
	lw.bodySize += size
	return size, err
}

func RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		uri := r.RequestURI
		method := r.Method
		lw := &loggedResponseWriter{ResponseWriter: w, statusCode: 200, bodySize: 0}
		h.ServeHTTP(lw, r)
		duration := strconv.FormatInt(time.Since(start).Milliseconds(), 10)
		Log.Info("got incoming HTTP request",
			"uri", uri,
			"method", method,
			"duration", duration+"ms")
		Log.Info("send HTTP response",
			"status", strconv.Itoa(lw.statusCode),
			"body size", strconv.Itoa(lw.bodySize))
	})

}
