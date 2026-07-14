package game

import (
	"context"
	"sync/atomic"
)

type State struct {
	clickCount atomic.Int64
	repository Repository
}

func NewState(repository Repository) *State {
	return &State{
		repository: repository,
	}
}

func (s *State) Load(ctx context.Context) error {
	if s.repository == nil {
		return nil
	}

	clicks, err := s.repository.LoadClicks(ctx)
	if err != nil {
		return err
	}

	s.clickCount.Store(clicks)
	return nil
}

func (s *State) Save(ctx context.Context) error {
	if s.repository == nil {
		return nil
	}

	clicks := s.clickCount.Load()
	return s.repository.SaveClicks(ctx, clicks)
}

func (s *State) IncrementClicks() int64 {
	return s.clickCount.Add(1)
}

func (s *State) GetClicks() int64 {
	return s.clickCount.Load()
}
