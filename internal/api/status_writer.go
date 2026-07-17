package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
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

func Logger(next http.Handler, logger *zap.Logger) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		fields := []zap.Field{
			zap.Int("status", rec.status),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", duration),
			zap.String("remote_addr", r.RemoteAddr),
		}

		switch {
		case rec.status >= http.StatusInternalServerError:
			logger.Error("http request completed", fields...)
		case rec.status >= http.StatusBadRequest:
			logger.Warn("http request completed", fields...)
		default:
			logger.Info("http request completed", fields...)
		}
	})
}
