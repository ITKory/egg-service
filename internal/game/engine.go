package game

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrRateLimited = errors.New("click rejected by rate limit")

type ClickResult struct {
	NewCount       int64
	UserClicks     int64
	TriggeredEvent *GameEvent
}

type GameEvent struct {
	Code    string
	Message string
}

type Engine struct {
	state      *State
	repository Repository

	mu          sync.RWMutex
	lastClickAt map[string]time.Time
}

func NewEngine(state *State, repository Repository) *Engine {
	return &Engine{
		state:       state,
		repository:  repository,
		lastClickAt: make(map[string]time.Time),
	}
}

func (e *Engine) ProcessClick(ctx context.Context, userID, username string) (*ClickResult, error) {
	if !e.allowClick(userID) {
		return nil, ErrRateLimited
	}

	userClicks, err := e.incrementUserClicks(ctx, userID, username)
	if err != nil {
		return nil, err
	}

	newCount := e.state.IncrementClicks()
	event := e.checkEvolution(newCount)

	return &ClickResult{
		NewCount:       newCount,
		UserClicks:     userClicks,
		TriggeredEvent: event,
	}, nil
}

func (e *Engine) incrementUserClicks(ctx context.Context, userID, username string) (int64, error) {
	if e.repository == nil {
		return 0, nil
	}

	return e.repository.IncrementUserClicks(ctx, userID, username, 1)
}

func (e *Engine) allowClick(userID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	lastTime, exists := e.lastClickAt[userID]
	now := time.Now()

	if exists && now.Sub(lastTime) < 50*time.Millisecond {
		return false
	}

	e.lastClickAt[userID] = now
	return true
}

func (e *Engine) checkEvolution(count int64) *GameEvent {
	if count > 0 && count%100 == 0 {
		return &GameEvent{
			Code:    "EVOLUTION",
			Message: "Яйцо начинает трескаться!",
		}
	}
	return nil
}

func (e *Engine) CleanupStaleEntries(maxAge time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for id, t := range e.lastClickAt {
		if now.Sub(t) > maxAge {
			delete(e.lastClickAt, id)
		}
	}
}
