package ws

import (
	"net/http"
	"testing"
)

func TestOriginChecker(t *testing.T) {
	check := originChecker([]string{"http://localhost:3000"})

	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "missing origin", host: "api.example.com", want: true},
		{name: "same host", origin: "https://api.example.com", host: "api.example.com", want: true},
		{name: "allowed origin", origin: "http://localhost:3000", host: "localhost:8080", want: true},
		{name: "disallowed origin", origin: "https://evil.example.com", host: "localhost:8080", want: false},
		{name: "invalid origin", origin: "://bad-origin", host: "localhost:8080", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &http.Request{
				Host:   tt.host,
				Header: http.Header{},
			}
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}

			if got := check(request); got != tt.want {
				t.Fatalf("originChecker() = %v, want %v", got, tt.want)
			}
		})
	}
}
