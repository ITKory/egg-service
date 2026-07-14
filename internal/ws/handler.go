package ws

import (
	"chaos-egg/internal/user"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Handler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		conn.SetReadLimit(512)

		userID := generateUserID()
		username := user.GenerateName()

		client := NewClient(hub, conn, userID, username)
		snapshot := hub.Register(client)
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

func generateUserID() string {
	raw := make([]byte, 3)
	if _, err := rand.Read(raw); err != nil {
		return "user_000000"
	}
	return "user_" + hex.EncodeToString(raw)
}
