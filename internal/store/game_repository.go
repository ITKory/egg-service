package store

import (
	"chaos-egg/internal/game"
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	gameStateKey        = "game:state"
	clicksField         = "clicks"
	leaderboardKey      = "leaderboard:global"
	leaderboardNamesKey = "leaderboard:names"
)

type GameRepository struct {
	rdb *redis.Client
}

func NewGameRepository(rdb *redis.Client) *GameRepository {
	return &GameRepository{rdb: rdb}
}

func (r *GameRepository) SaveClicks(ctx context.Context, clicks int64) error {
	return r.rdb.HSet(ctx, gameStateKey, clicksField, clicks).Err()
}

func (r *GameRepository) LoadClicks(ctx context.Context) (int64, error) {
	val, err := r.rdb.HGet(ctx, gameStateKey, clicksField).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(val, 10, 64)
}

func (r *GameRepository) IncrementUserClicks(ctx context.Context, userID, username string, delta int64) (int64, error) {
	if username != "" {
		if err := r.rdb.HSet(ctx, leaderboardNamesKey, userID, username).Err(); err != nil {
			return 0, err
		}
	}

	score, err := r.rdb.ZIncrBy(ctx, leaderboardKey, float64(delta), userID).Result()
	if err != nil {
		return 0, err
	}

	return int64(score), nil
}

func (r *GameRepository) UpdateLeaderboard(ctx context.Context, userID, username string, clicks int64) error {
	if username != "" {
		if err := r.rdb.HSet(ctx, leaderboardNamesKey, userID, username).Err(); err != nil {
			return err
		}
	}

	return r.rdb.ZAdd(ctx, leaderboardKey, redis.Z{
		Score:  float64(clicks),
		Member: userID,
	}).Err()
}

func (r *GameRepository) GetLeaderboard(ctx context.Context, top int) ([]game.LeaderboardEntry, error) {
	results, err := r.rdb.ZRevRangeWithScores(ctx, leaderboardKey, 0, int64(top-1)).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]game.LeaderboardEntry, 0, len(results))
	for i, z := range results {
		userID, ok := z.Member.(string)
		if !ok {
			continue
		}
		username, err := r.rdb.HGet(ctx, leaderboardNamesKey, userID).Result()
		if err == redis.Nil {
			username = ""
		} else if err != nil {
			return nil, err
		}

		entries = append(entries, game.LeaderboardEntry{
			Rank:     i + 1,
			UserID:   userID,
			Username: username,
			Score:    int64(z.Score),
		})
	}

	return entries, nil
}
