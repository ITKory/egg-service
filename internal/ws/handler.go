package ws

import (
	"chaos-egg/internal/user"
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type HandlerConfig struct {
	AllowedOrigins []string
}

func Handler(hub *Hub, config HandlerConfig) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     originChecker(config.AllowedOrigins),
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		userID := generateUserID()
		username := user.GenerateName()

		client := NewClient(hub, conn, userID, username)
		registerCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := hub.Register(registerCtx, client)
		if err != nil {
			log.Printf("WebSocket register error: %v", err)
			_ = conn.Close()
			return
		}
		presence := NewPresencePayload(snapshot)

		client.Send(Message{
			Type: MessageWelcome,
			Data: WelcomePayload{
				UserID:         userID,
				Username:       username,
				ConnectedUsers: presence.ConnectedUsers,
				Users:          presence.Users,
			},
		})

		_ = hub.BroadcastJSON(Message{
			Type: MessageUserConnected,
			Data: UserPayload{
				UserID:         userID,
				Username:       username,
				ConnectedUsers: presence.ConnectedUsers,
				Users:          presence.Users,
			},
		})

		log.Printf("Client connected: %s (%s)", username, userID)

		go client.ReadPump()
		go client.WritePump()
	}
}

func originChecker(allowedOrigins []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := normalizeOrigin(origin)
		if normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}

	if len(allowed) == 0 {
		return func(*http.Request) bool {
			return true
		}
	}

	return func(r *http.Request) bool {
		rawOrigin := strings.TrimSpace(r.Header.Get("Origin"))
		if rawOrigin == "" {
			return true
		}

		origin := normalizeOrigin(rawOrigin)
		if origin == "" {
			return false
		}
		if sameHost(origin, r.Host) {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}

func normalizeOrigin(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func sameHost(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, host)
}

func generateUserID() string {
	raw := make([]byte, 3)
	if _, err := rand.Read(raw); err != nil {
		return "user_000000"
	}
	return "user_" + hex.EncodeToString(raw)
}
