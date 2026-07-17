package game

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type ActiveEvent struct {
	Definition EventDefinition
	ExpiresAt  time.Time
}

type EventNotification struct {
	Type            string
	Code            EventCode
	Name            string
	Message         string
	DurationSeconds float64
}

type EventPublisher interface {
	PublishEvent(EventNotification) error
}

type Logger interface {
	Info(message string)
	Error(message string, err error)
}

type noopLogger struct{}

func (noopLogger) Info(string)         {}
func (noopLogger) Error(string, error) {}

type EventEngine struct {
	publisher   EventPublisher
	logger      Logger
	mu          sync.RWMutex
	activeEvent *ActiveEvent
}

func NewEventEngine(publisher EventPublisher, loggers ...Logger) *EventEngine {
	logger := Logger(noopLogger{})
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}

	return &EventEngine{
		publisher: publisher,
		logger:    logger,
	}
}

func (e *EventEngine) Run(ctx context.Context) {
	e.logger.Info("event engine started")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("event engine stopping")
			return

		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *EventEngine) tick() {
	e.mu.Lock()
	if e.activeEvent != nil {
		if time.Now().After(e.activeEvent.ExpiresAt) {
			e.mu.Unlock()
			e.endEvent()
		} else {
			e.mu.Unlock()
		}
		return
	}
	e.mu.Unlock()

	if rand.Float64() > 0.2 {
		return
	}

	e.startRandomEvent()
}

func (e *EventEngine) startRandomEvent() {
	idx := rand.Intn(len(EventRegistry))
	def := EventRegistry[idx]

	expiresAt := time.Now().Add(def.Duration)

	e.mu.Lock()
	e.activeEvent = &ActiveEvent{
		Definition: def,
		ExpiresAt:  expiresAt,
	}
	e.mu.Unlock()

	e.logger.Info(fmt.Sprintf("event started: %s (ends at %s)", def.Name, expiresAt.Format("15:04:05")))

	e.publish(EventNotification{
		Type:            "event_start",
		Code:            def.Code,
		Name:            def.Name,
		Message:         def.Message,
		DurationSeconds: def.Duration.Seconds(),
	})
}

func (e *EventEngine) endEvent() {
	e.mu.Lock()
	if e.activeEvent == nil {
		e.mu.Unlock()
		return
	}
	name := e.activeEvent.Definition.Name
	code := e.activeEvent.Definition.Code
	e.activeEvent = nil
	e.mu.Unlock()

	e.logger.Info(fmt.Sprintf("event ended: %s", name))

	e.publish(EventNotification{
		Type: "event_end",
		Code: code,
	})
}

func (e *EventEngine) GetActiveEvent() *ActiveEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.activeEvent == nil {
		return nil
	}
	cp := *e.activeEvent
	return &cp
}

func (e *EventEngine) publish(notification EventNotification) {
	if e.publisher == nil {
		return
	}

	if err := e.publisher.PublishEvent(notification); err != nil {
		e.logger.Error("event publish failed", err)
	}
}
