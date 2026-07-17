package api

import (
	"chaos-egg/internal/game"
	"chaos-egg/internal/logging"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

type StateReader interface {
	GetClicks() int64
}

type EventReader interface {
	GetActiveEvent() *game.ActiveEvent
}

type LeaderboardReader interface {
	GetLeaderboard(ctx context.Context, top int) ([]game.LeaderboardEntry, error)
}

type ClickProcessor interface {
	ProcessClick(ctx context.Context, userID, username string) (*game.ClickResult, error)
}

type PresenceReader interface {
	Presence(ctx context.Context) (game.PresenceSnapshot, error)
}

type Handlers struct {
	state       StateReader
	events      EventReader
	leaderboard LeaderboardReader
	clicks      ClickProcessor
	presence    PresenceReader
	logger      *zap.Logger
}

func NewHandlers(
	state StateReader,
	events EventReader,
	leaderboard LeaderboardReader,
	clicks ClickProcessor,
	presence PresenceReader,
	logger *zap.Logger,
) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Handlers{
		state:       state,
		events:      events,
		leaderboard: leaderboard,
		clicks:      clicks,
		presence:    presence,
		logger:      logger,
	}
}

type stateResponse struct {
	Clicks         int64                   `json:"clicks"`
	ActiveEvent    *game.ActiveEvent       `json:"activeEvent"`
	ConnectedUsers int                     `json:"connectedUsers"`
	Users          []connectedUserResponse `json:"users"`
}

type leaderboardResponse struct {
	Leaderboard []game.LeaderboardEntry `json:"leaderboard"`
}

type clickResponse struct {
	Clicks int64 `json:"clicks"`
}

type connectedUserResponse struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

func (h *Handlers) GetState(w http.ResponseWriter, r *http.Request) {
	presence, err := h.presence.Presence(r.Context())
	if err != nil {
		h.logger.Error("failed to get presence",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelRecoverable),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, stateResponse{
		Clicks:         h.state.GetClicks(),
		ActiveEvent:    h.events.GetActiveEvent(),
		ConnectedUsers: presence.ConnectedUsers,
		Users:          connectedUserResponses(presence.Users),
	})
}

func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.leaderboard.GetLeaderboard(r.Context(), 10)
	if err != nil {
		h.logger.Error("failed to get leaderboard",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelRecoverable),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, leaderboardResponse{Leaderboard: entries})
}

func (h *Handlers) Click(w http.ResponseWriter, r *http.Request) {
	result, err := h.clicks.ProcessClick(r.Context(), "http_user", "HTTP user")
	if errors.Is(err, game.ErrRateLimited) {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	if err != nil {
		h.logger.Error("failed to process click",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelRecoverable),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, clickResponse{Clicks: result.NewCount})
}

func connectedUserResponses(users []game.ConnectedUser) []connectedUserResponse {
	response := make([]connectedUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, connectedUserResponse{
			UserID:   user.UserID,
			Username: user.Username,
		})
	}

	return response
}

func (h *Handlers) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode json response",
			logging.Error(err),
			logging.ErrorLevelField(logging.ErrorLevelRecoverable),
		)
	}
}
