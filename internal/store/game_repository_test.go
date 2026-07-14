package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRepository(t *testing.T) *GameRepository {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	return NewGameRepository(rdb)
}

func TestGameRepository_SaveAndLoadClicks(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	if err := repository.SaveClicks(ctx, 42); err != nil {
		t.Fatalf("SaveClicks failed: %v", err)
	}

	loaded, err := repository.LoadClicks(ctx)
	if err != nil {
		t.Fatalf("LoadClicks failed: %v", err)
	}

	if loaded != 42 {
		t.Errorf("Expected 42, got %d", loaded)
	}
}

func TestGameRepository_Leaderboard(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	if err := repository.UpdateLeaderboard(ctx, "user_1", "Alice", 10); err != nil {
		t.Fatalf("UpdateLeaderboard user_1 failed: %v", err)
	}
	if err := repository.UpdateLeaderboard(ctx, "user_2", "Bob", 30); err != nil {
		t.Fatalf("UpdateLeaderboard user_2 failed: %v", err)
	}
	if err := repository.UpdateLeaderboard(ctx, "user_3", "Carol", 20); err != nil {
		t.Fatalf("UpdateLeaderboard user_3 failed: %v", err)
	}

	entries, err := repository.GetLeaderboard(ctx, 2)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
	if entries[0].UserID != "user_2" || entries[0].Username != "Bob" || entries[0].Score != 30 {
		t.Errorf("First place should be user_2 with 30 clicks")
	}
	if entries[1].UserID != "user_3" || entries[1].Username != "Carol" || entries[1].Score != 20 {
		t.Errorf("Second place should be user_3 with 20 clicks")
	}
}

func TestGameRepository_IncrementUserClicks(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	clicks, err := repository.IncrementUserClicks(ctx, "user_1", "Alice", 1)
	if err != nil {
		t.Fatalf("IncrementUserClicks failed: %v", err)
	}
	if clicks != 1 {
		t.Fatalf("Expected first increment to return 1, got %d", clicks)
	}

	clicks, err = repository.IncrementUserClicks(ctx, "user_1", "Alice", 1)
	if err != nil {
		t.Fatalf("IncrementUserClicks failed: %v", err)
	}
	if clicks != 2 {
		t.Fatalf("Expected second increment to return 2, got %d", clicks)
	}
}
