package ws

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBroadcastPresenceRebroadcastsAfterRemovingBlockedClient(t *testing.T) {
	hub := NewHub(make(chan *Message, 1))
	active := &Client{
		send:     make(chan []byte, 2),
		UserID:   "active",
		Username: "Active User",
	}
	blocked := &Client{
		send:     make(chan []byte),
		UserID:   "blocked",
		Username: "Blocked User",
	}

	hub.clients[active] = true
	hub.clients[blocked] = true

	hub.broadcastPresence(hub.presenceSnapshot())

	first := readPresencePayload(t, <-active.send)
	second := readPresencePayload(t, <-active.send)

	if first.ConnectedUsers != 2 {
		t.Fatalf("first presence connected users = %d, want 2", first.ConnectedUsers)
	}
	if second.ConnectedUsers != 1 {
		t.Fatalf("second presence connected users = %d, want 1", second.ConnectedUsers)
	}
	if _, ok := hub.clients[blocked]; ok {
		t.Fatal("blocked client was not removed")
	}
}

func BenchmarkHub_Broadcast(b *testing.B) {
	eventCh := make(chan *Message, 100)
	hub := NewHub(eventCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	fakeClients := make([]*Client, 1000)
	for i := 0; i < 1000; i++ {
		c := &Client{
			hub:      hub,
			send:     make(chan []byte, 100),
			UserID:   "bench_user",
			Username: "Bench User",
		}
		fakeClients[i] = c
		if _, err := hub.Register(ctx, c); err != nil {
			b.Fatalf("Register failed: %v", err)
		}
	}

	msg := []byte(`{"type":"test","data":"load"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := hub.Broadcast(msg); err != nil {
			b.Fatalf("Broadcast failed: %v", err)
		}
	}

	_ = fakeClients
}

func readPresencePayload(t *testing.T, data []byte) PresencePayload {
	t.Helper()

	var message struct {
		Type MessageType     `json:"type"`
		Data PresencePayload `json:"data"`
	}
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("decode presence message: %v", err)
	}
	if message.Type != MessagePresence {
		t.Fatalf("message type = %s, want %s", message.Type, MessagePresence)
	}
	return message.Data
}
