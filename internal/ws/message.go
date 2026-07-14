package ws

import (
	"bytes"
	"chaos-egg/internal/game"
	"encoding/json"
	"sync"
)

type MessageType string

const (
	MessageClick            MessageType = "click"
	MessageEmote            MessageType = "emote"
	MessageEvent            MessageType = "event"
	MessageEventStart       MessageType = "event_start"
	MessageEventEnd         MessageType = "event_end"
	MessagePresence         MessageType = "presence_update"
	MessageState            MessageType = "state_update"
	MessageWelcome          MessageType = "welcome"
	MessageUserConnected    MessageType = "user_connected"
	MessageUserDisconnected MessageType = "user_disconnected"
)

type Message struct {
	Type     MessageType `json:"type"`
	UserID   string      `json:"userId,omitempty"`
	Username string      `json:"username,omitempty"`
	Data     any         `json:"data,omitempty"`
}

type StatePayload struct {
	ClickCount     int64                   `json:"clickCount"`
	UserClicks     int64                   `json:"userClicks,omitempty"`
	ConnectedUsers int                     `json:"connectedUsers"`
	Users          []ConnectedUserPayload  `json:"users,omitempty"`
	Leaderboard    []game.LeaderboardEntry `json:"leaderboard,omitempty"`
	UserID         string                  `json:"userId,omitempty"`
	Username       string                  `json:"username,omitempty"`
}

type WelcomePayload struct {
	UserID         string                 `json:"userId"`
	Username       string                 `json:"username"`
	ConnectedUsers int                    `json:"connectedUsers"`
	Users          []ConnectedUserPayload `json:"users"`
}

type UserPayload struct {
	UserID         string                 `json:"userId"`
	Username       string                 `json:"username"`
	ConnectedUsers int                    `json:"connectedUsers"`
	Users          []ConnectedUserPayload `json:"users,omitempty"`
}

type EmotePayload struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Emote    string `json:"emote"`
}

type EventPayload struct {
	Code     string  `json:"code"`
	Name     string  `json:"name,omitempty"`
	Message  string  `json:"message,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

type ConnectedUserPayload struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type PresencePayload struct {
	ConnectedUsers int                    `json:"connectedUsers"`
	Users          []ConnectedUserPayload `json:"users"`
}

func NewPresencePayload(snapshot game.PresenceSnapshot) PresencePayload {
	users := make([]ConnectedUserPayload, 0, len(snapshot.Users))
	for _, user := range snapshot.Users {
		users = append(users, ConnectedUserPayload{
			UserID:   user.UserID,
			Username: user.Username,
		})
	}

	return PresencePayload{
		ConnectedUsers: snapshot.ConnectedUsers,
		Users:          users,
	}
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func Encode(msg Message) ([]byte, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	defer bufPool.Put(buf)

	err := json.NewEncoder(buf).Encode(msg)
	if err != nil {
		return nil, err
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}

func Decode(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
