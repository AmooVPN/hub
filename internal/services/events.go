package services

import (
	"context"
	"sync"
	"time"
)

type Event struct {
	Type       string
	ActorType  string
	ActorID    *int64
	TargetType string
	TargetID   *int64
	Payload    map[string]any
	CreatedAt  time.Time
}

type EventHandler func(context.Context, Event)

type EventBus interface {
	Publish(context.Context, Event)
	Subscribe(string, EventHandler)
}

type SimpleEventBus struct {
	mu   sync.RWMutex
	subs map[string][]EventHandler
}

func NewEventBus() *SimpleEventBus { return &SimpleEventBus{subs: make(map[string][]EventHandler)} }

func (b *SimpleEventBus) Publish(ctx context.Context, event Event) {
	if b == nil {
		return
	}
	b.mu.RLock()
	handlers := append([]EventHandler(nil), b.subs[event.Type]...)
	b.mu.RUnlock()
	for _, handler := range handlers {
		if handler != nil {
			handler(ctx, event)
		}
	}
}

func (b *SimpleEventBus) Subscribe(eventType string, handler EventHandler) {
	if b == nil || handler == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], handler)
}
