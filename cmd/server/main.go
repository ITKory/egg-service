package main

import (
	"chaos-egg/internal/api"
	"chaos-egg/internal/game"
	"chaos-egg/internal/logging"
	"chaos-egg/internal/store"
	"chaos-egg/internal/ws"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	logger, err := logging.New(logging.FromEnv())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to configure logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := store.NewRedisClient(ctx, redisAddr, os.Getenv("REDIS_PASSWORD"), 0)
	if err != nil {
		logger.Fatal("failed to connect to redis",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelFatal),
			zap.String("redis_addr", redisAddr),
		)
	}
	defer redisClient.Close()

	gameRepository := store.NewGameRepository(redisClient.Client())
	state := game.NewState(gameRepository)

	if err := state.Load(ctx); err != nil {
		logger.Warn("failed to load state",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelRecoverable),
		)
	}

	engine := game.NewEngine(state, gameRepository)

	eventCh := make(chan *ws.Message, 100)

	hub := ws.NewHub(eventCh, logger.Named("ws"))
	go hub.Run(ctx)
	go runGameEngine(ctx, engine, state, gameRepository, hub, eventCh, logger.Named("engine"))

	eventEngine := game.NewEventEngine(wsEventPublisher{hub: hub}, gameLogger{logger: logger.Named("events")})
	go eventEngine.Run(ctx)

	handlers := api.NewHandlers(state, eventEngine, gameRepository, engine, hub, logger.Named("api"))
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
	handler = api.Logger(handler, logger.Named("http"))

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		logger.Info("server starting", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server listen failed",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelFatal),
				zap.String("addr", server.Addr),
			)
		}
	}()

	<-shutdown
	logger.Info("server shutting down")

	cancel()

	if err := state.Save(context.Background()); err != nil {
		logger.Error("failed to save state",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelCritical),
		)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelCritical),
		)
	}

	logger.Info("server stopped")
}

func websocketAllowedOrigins() []string {
	raw := os.Getenv("WS_ALLOWED_ORIGINS")
	if raw == "" {
		return nil
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

type gameLogger struct {
	logger *zap.Logger
}

func (l gameLogger) Info(message string) {
	l.logger.Info(message)
}

func (l gameLogger) Error(message string, err error) {
	l.logger.Error(message,
		logging.Error(err),
		logging.ErrorLevelField(logging.ErrorLevelRecoverable),
	)
}

func runGameEngine(
	ctx context.Context,
	engine *game.Engine,
	state *game.State,
	leaderboard game.Repository,
	hub *ws.Hub,
	events <-chan *ws.Message,
	logger *zap.Logger,
) {
	logger.Info("game engine started")
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
				logger.Error("periodic save failed",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelRecoverable),
				)
			}

		case <-cleanupTicker.C:
			engine.CleanupStaleEntries(10 * time.Minute)

		case msg := <-events:
			handleEvent(ctx, msg, engine, leaderboard, hub, logger)
		}
	}
}

func handleEvent(ctx context.Context, msg *ws.Message, engine *game.Engine, leaderboard game.Repository, hub *ws.Hub, logger *zap.Logger) {
	if msg.UserID == "" {
		logger.Warn("message without user id",
			logging.ErrorLevelField(logging.ErrorLevelExpected),
			zap.String("message_type", string(msg.Type)),
		)
		return
	}

	switch msg.Type {
	case ws.MessageClick:
		result, err := engine.ProcessClick(ctx, msg.UserID, msg.Username)
		if errors.Is(err, game.ErrRateLimited) {
			return
		}
		if err != nil {
			logger.Error("failed to process websocket click",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
				zap.String("user_id", msg.UserID),
			)
			return
		}

		entries, err := leaderboard.GetLeaderboard(ctx, 10)
		if err != nil {
			logger.Error("failed to get leaderboard",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
			)
		}
		snapshot, err := hub.Presence(ctx)
		if err != nil {
			logger.Error("failed to get presence snapshot",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
			)
		}
		presence := ws.NewPresencePayload(snapshot)

		if err := hub.BroadcastJSON(ws.Message{
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
		}); err != nil {
			logger.Error("failed to broadcast state update",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
				zap.String("user_id", msg.UserID),
			)
		}

		if result.TriggeredEvent != nil {
			logger.Info("event triggered",
				zap.String("event_message", result.TriggeredEvent.Message),
				zap.String("user_id", msg.UserID),
			)
			if err := hub.BroadcastJSON(ws.Message{
				Type: ws.MessageEvent,
				Data: ws.EventPayload{
					Code:    result.TriggeredEvent.Code,
					Message: result.TriggeredEvent.Message,
				},
			}); err != nil {
				logger.Error("failed to broadcast triggered event",
					logging.Error(err),
					logging.ErrorLevelField(logging.ErrorLevelRecoverable),
					zap.String("user_id", msg.UserID),
				)
			}
		}

	case ws.MessageEmote:
		emote := emoteFromMessageData(msg.Data)
		if emote == "" {
			return
		}

		if err := hub.BroadcastJSON(ws.Message{
			Type:   ws.MessageEmote,
			UserID: msg.UserID,
			Data: ws.EmotePayload{
				UserID:   msg.UserID,
				Username: msg.Username,
				Emote:    emote,
			},
		}); err != nil {
			logger.Error("failed to broadcast emote",
				logging.Error(err),
				logging.ErrorLevelField(logging.ErrorLevelRecoverable),
				zap.String("user_id", msg.UserID),
			)
		}
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
