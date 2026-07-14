package api

import (
	"chaos-egg/internal/game"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
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
}

func NewHandlers(
	state StateReader,
	events EventReader,
	leaderboard LeaderboardReader,
	clicks ClickProcessor,
	presence PresenceReader,
) *Handlers {
	return &Handlers{
		state:       state,
		events:      events,
		leaderboard: leaderboard,
		clicks:      clicks,
		presence:    presence,
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
		log.Printf("Failed to get presence: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stateResponse{
		Clicks:         h.state.GetClicks(),
		ActiveEvent:    h.events.GetActiveEvent(),
		ConnectedUsers: presence.ConnectedUsers,
		Users:          connectedUserResponses(presence.Users),
	})
}

func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.leaderboard.GetLeaderboard(r.Context(), 10)
	if err != nil {
		log.Printf("Failed to get leaderboard: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, leaderboardResponse{Leaderboard: entries})
}

func (h *Handlers) Click(w http.ResponseWriter, r *http.Request) {
	result, err := h.clicks.ProcessClick(r.Context(), "http_user", "HTTP user")
	if errors.Is(err, game.ErrRateLimited) {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	if err != nil {
		log.Printf("Failed to process click: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, clickResponse{Clicks: result.NewCount})
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}
