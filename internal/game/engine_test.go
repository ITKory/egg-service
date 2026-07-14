package game

import (
	"context"
	"errors"
	"testing"
)

func TestEngine_ProcessClick_RateLimit(t *testing.T) {
	state := NewState(nil)
	engine := NewEngine(state, nil)
	userID := "test_user"

	result, err := engine.ProcessClick(context.Background(), userID, "Test User")
	if err != nil || result == nil {
		t.Fatal("First click should not be rate limited")
	}

	result, err = engine.ProcessClick(context.Background(), userID, "Test User")

	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Expected rate limit error, got %v", err)
	}
	if result != nil {
		t.Fatalf("Expected empty result for rate-limited click, got %v", result)
	}
}

func TestEngine_ProcessClick_Evolution(t *testing.T) {
	tests := []struct {
		name         string
		clicksToMake int
		expectEvent  bool
		expectedCode string
	}{
		{
			name:         "No event at 50 clicks",
			clicksToMake: 50,
			expectEvent:  false,
		},
		{
			name:         "Event triggers at 100 clicks",
			clicksToMake: 100,
			expectEvent:  true,
			expectedCode: "EVOLUTION",
		},
		{
			name:         "Event triggers at 200 clicks",
			clicksToMake: 200,
			expectEvent:  true,
			expectedCode: "EVOLUTION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState(nil)
			engine := NewEngine(state, nil)

			var lastResult *ClickResult
			for i := 1; i <= tt.clicksToMake; i++ {
				userID := "user_" + string(rune(i))
				result, err := engine.ProcessClick(context.Background(), userID, "Test User")
				if err != nil {
					t.Fatalf("ProcessClick failed: %v", err)
				}
				lastResult = result
			}

			if tt.expectEvent {
				if lastResult == nil || lastResult.TriggeredEvent == nil {
					t.Errorf("Expected event to be triggered, but got nil")
				} else if lastResult.TriggeredEvent.Code != tt.expectedCode {
					t.Errorf("Expected event code %s, got %s", tt.expectedCode, lastResult.TriggeredEvent.Code)
				}
			} else {
				if lastResult != nil && lastResult.TriggeredEvent != nil {
					t.Errorf("Expected no event, but got %s", lastResult.TriggeredEvent.Code)
				}
			}
		})
	}
}
