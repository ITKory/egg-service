package main

import (
	"chaos-egg/internal/api"
	"chaos-egg/internal/game"
	"chaos-egg/internal/store"
	"chaos-egg/internal/ws"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := store.NewRedisClient(ctx, redisAddr, os.Getenv("REDIS_PASSWORD"), 0)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	gameRepository := store.NewGameRepository(redisClient.Client())
	state := game.NewState(gameRepository)

	if err := state.Load(ctx); err != nil {
		log.Printf("Warning: failed to load state: %v", err)
	}

	engine := game.NewEngine(state, gameRepository)

	eventCh := make(chan *ws.Message, 100)

	hub := ws.NewHub(eventCh)
	go hub.Run(ctx)
	go runGameEngine(ctx, engine, state, gameRepository, hub, eventCh)

	eventEngine := game.NewEventEngine(wsEventPublisher{hub: hub})
	go eventEngine.Run(ctx)

	handlers := api.NewHandlers(state, eventEngine, gameRepository, engine, hub)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/state", handlers.GetState)
	mux.HandleFunc("/api/leaderboard", handlers.GetLeaderboard)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/click", handlers.Click)
	mux.HandleFunc("/ws", ws.Handler(hub, ws.HandlerConfig{
		AllowedOrigins: websocketAllowedOrigins(),
	}))

	var handler http.Handler = mux

	handler = api.CORSMiddleware(handler)
	handler = api.Logger(handler)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-shutdown
	log.Println("Shutting down...")

	cancel()

	if err := state.Save(context.Background()); err != nil {
		log.Printf("Failed to save state: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func websocketAllowedOrigins() []string {
	raw := os.Getenv("WS_ALLOWED_ORIGINS")
	if raw == "" {
		return []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

type wsEventPublisher struct {
	hub *ws.Hub
}

func (p wsEventPublisher) PublishEvent(notification game.EventNotification) error {
	return p.hub.BroadcastJSON(ws.Message{
		Type: ws.MessageType(notification.Type),
		Data: ws.EventPayload{
			Code:     string(notification.Code),
			Name:     notification.Name,
			Message:  notification.Message,
			Duration: notification.DurationSeconds,
		},
	})
}

func runGameEngine(
	ctx context.Context,
	engine *game.Engine,
	state *game.State,
	leaderboard game.Repository,
	hub *ws.Hub,
	events <-chan *ws.Message,
) {
	log.Println("Game Engine started")
	saveTicker := time.NewTicker(10 * time.Second)
	defer saveTicker.Stop()
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-saveTicker.C:
			if err := state.Save(ctx); err != nil {
				log.Printf("Periodic save failed: %v", err)
			}

		case <-cleanupTicker.C:
			engine.CleanupStaleEntries(10 * time.Minute)

		case msg := <-events:
			handleEvent(ctx, msg, engine, leaderboard, hub)
		}
	}
}

func handleEvent(ctx context.Context, msg *ws.Message, engine *game.Engine, leaderboard game.Repository, hub *ws.Hub) {
	if msg.UserID == "" {
		log.Printf("Message without UserID: %v", msg)
		return
	}

	switch msg.Type {
	case ws.MessageClick:
		result, err := engine.ProcessClick(ctx, msg.UserID, msg.Username)
		if errors.Is(err, game.ErrRateLimited) {
			return
		}
		if err != nil {
			log.Printf("Failed to process click: %v", err)
			return
		}

		entries, err := leaderboard.GetLeaderboard(ctx, 10)
		if err != nil {
			log.Printf("Failed to get leaderboard: %v", err)
		}
		snapshot, err := hub.Presence(ctx)
		if err != nil {
			log.Printf("Failed to get presence snapshot: %v", err)
		}
		presence := ws.NewPresencePayload(snapshot)

		_ = hub.BroadcastJSON(ws.Message{
			Type:   ws.MessageState,
			UserID: msg.UserID,
			Data: ws.StatePayload{
				ClickCount:     result.NewCount,
				UserClicks:     result.UserClicks,
				ConnectedUsers: presence.ConnectedUsers,
				Users:          presence.Users,
				Leaderboard:    entries,
				UserID:         msg.UserID,
				Username:       msg.Username,
			},
		})

		if result.TriggeredEvent != nil {
			log.Printf("EVENT TRIGGERED: %s", result.TriggeredEvent.Message)
			_ = hub.BroadcastJSON(ws.Message{
				Type: ws.MessageEvent,
				Data: ws.EventPayload{
					Code:    result.TriggeredEvent.Code,
					Message: result.TriggeredEvent.Message,
				},
			})
		}

	case ws.MessageEmote:
		emote := emoteFromMessageData(msg.Data)
		if emote == "" {
			return
		}

		_ = hub.BroadcastJSON(ws.Message{
			Type:   ws.MessageEmote,
			UserID: msg.UserID,
			Data: ws.EmotePayload{
				UserID:   msg.UserID,
				Username: msg.Username,
				Emote:    emote,
			},
		})
	}
}

func emoteFromMessageData(data any) string {
	payload, ok := data.(map[string]any)
	if !ok {
		return ""
	}

	if emote, ok := payload["emote"].(string); ok {
		return emote
	}
	if reaction, ok := payload["reaction"].(string); ok {
		return reaction
	}
	if emoji, ok := payload["emoji"].(string); ok {
		return emoji
	}
	return ""
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
