package api

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// Hijack пробрасывает поддержку WebSocket upgrade через обертку логгера.
func (rec *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rec.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer %T does not implement http.Hijacker", rec.ResponseWriter)
	}

	rec.status = http.StatusSwitchingProtocols
	return hijacker.Hijack()
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		log.Printf("[%d] %s %s %v", rec.status, r.Method, r.URL.Path, duration)
	})
}
