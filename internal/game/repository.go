package game

import "context"

type Repository interface {
	SaveClicks(ctx context.Context, clicks int64) error
	LoadClicks(ctx context.Context) (int64, error)
	IncrementUserClicks(ctx context.Context, userID, username string, delta int64) (int64, error)
	UpdateLeaderboard(ctx context.Context, userID, username string, clicks int64) error
	GetLeaderboard(ctx context.Context, top int) ([]LeaderboardEntry, error)
}

type LeaderboardEntry struct {
	Rank     int    `json:"rank"`
	UserID   string `json:"userId"`
	Username string `json:"username,omitempty"`
	Score    int64  `json:"score"`
}
