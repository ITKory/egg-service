package ws

import (
	"chaos-egg/internal/game"
	"context"
	"errors"

	"go.uber.org/zap"
)

var ErrBroadcastQueueFull = errors.New("websocket hub broadcast queue is full")

type registerRequest struct {
	client *Client
	done   chan game.PresenceSnapshot
}

type Hub struct {
	clients    map[*Client]bool
	register   chan registerRequest
	unregister chan *Client
	broadcast  chan []byte
	presence   chan chan game.PresenceSnapshot

	events chan<- *Message
	logger *zap.Logger
}

func NewHub(eventCh chan<- *Message, loggers ...*zap.Logger) *Hub {
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}

	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan registerRequest, 32),
		unregister: make(chan *Client, 32),
		broadcast:  make(chan []byte, 100),
		presence:   make(chan chan game.PresenceSnapshot, 16),
		events:     eventCh,
		logger:     logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return

		case request := <-h.register:
			h.clients[request.client] = true
			snapshot := h.presenceSnapshot()
			h.broadcastPresence(snapshot)
			request.done <- snapshot

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.broadcastPresence(h.presenceSnapshot())
			}

		case message := <-h.broadcast:
			if h.broadcastToClients(message) {
				h.broadcastPresence(h.presenceSnapshot())
			}

		case reply := <-h.presence:
			reply <- h.presenceSnapshot()
		}
	}
}

func (h *Hub) Register(ctx context.Context, client *Client) (game.PresenceSnapshot, error) {
	done := make(chan game.PresenceSnapshot, 1)
	select {
	case h.register <- registerRequest{client: client, done: done}:
	case <-ctx.Done():
		return game.PresenceSnapshot{}, ctx.Err()
	}

	select {
	case snapshot := <-done:
		return snapshot, nil
	case <-ctx.Done():
		return game.PresenceSnapshot{}, ctx.Err()
	}
}

func (h *Hub) Presence(ctx context.Context) (game.PresenceSnapshot, error) {
	reply := make(chan game.PresenceSnapshot, 1)
	select {
	case h.presence <- reply:
	case <-ctx.Done():
		return game.PresenceSnapshot{}, ctx.Err()
	}

	select {
	case snapshot := <-reply:
		return snapshot, nil
	case <-ctx.Done():
		return game.PresenceSnapshot{}, ctx.Err()
	}
}

func (h *Hub) Broadcast(data []byte) error {
	select {
	case h.broadcast <- data:
		return nil
	default:
		return ErrBroadcastQueueFull
	}
}

func (h *Hub) BroadcastJSON(msg Message) error {
	data, err := Encode(msg)
	if err != nil {
		return err
	}
	return h.Broadcast(data)
}

func (h *Hub) broadcastPresence(snapshot game.PresenceSnapshot) {
	for {
		data, err := Encode(Message{
			Type: MessagePresence,
			Data: NewPresencePayload(snapshot),
		})
		if err != nil {
			return
		}

		if !h.broadcastToClients(data) {
			return
		}
		snapshot = h.presenceSnapshot()
	}
}

func (h *Hub) broadcastToClients(message []byte) bool {
	removedClient := false
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
			removedClient = true
		}
	}

	return removedClient
}

func (h *Hub) presenceSnapshot() game.PresenceSnapshot {
	users := make([]game.ConnectedUser, 0, len(h.clients))
	for client := range h.clients {
		users = append(users, game.ConnectedUser{
			UserID:   client.UserID,
			Username: client.Username,
		})
	}

	return game.PresenceSnapshot{
		ConnectedUsers: len(users),
		Users:          users,
	}
}
