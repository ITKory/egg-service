package game

import (
	"context"
	"log"
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

type EventEngine struct {
	publisher   EventPublisher
	mu          sync.RWMutex
	activeEvent *ActiveEvent
}

func NewEventEngine(publisher EventPublisher) *EventEngine {
	return &EventEngine{
		publisher: publisher,
	}
}

func (e *EventEngine) Run(ctx context.Context) {
	log.Println("Event Engine started")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Event Engine stopping...")
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

	log.Printf("EVENT STARTED: %s (ends at %s)", def.Name, expiresAt.Format("15:04:05"))

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

	log.Printf("EVENT ENDED: %s", name)

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
		log.Printf("Event publish failed: %v", err)
	}
}
