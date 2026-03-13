package event

import (
	"sync"
	"time"
)

type EventType int

const (
	TypeRequestStart EventType = iota
	TypeRequestEnd
	TypeDetection
	TypeConfigChange
)

type Event struct {
	Type      EventType
	Timestamp time.Time
	ReqID     string
	Data      interface{}
}

type RequestData struct {
	Method string
	URL    string
	Host   string
}

type RequestEndData struct {
	Status   int
	Duration time.Duration
}

type DetectionData struct {
	Context  string
	RuleType string
	Original string
	Action   string
}

type Bus struct {
	subscribers []chan Event
	mu          sync.RWMutex
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make([]chan Event, 0),
	}
}

func (b *Bus) Subscribe() chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 100)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	for _, ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// Drop event if subscriber is slow
		}
	}
}

// Global bus instance
var GlobalBus = NewBus()
