package ws

import (
	"context"
	"testing"
)

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
		hub.Register(c)
	}

	msg := []byte(`{"type":"test","data":"load"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(msg)
	}

	_ = fakeClients
}
