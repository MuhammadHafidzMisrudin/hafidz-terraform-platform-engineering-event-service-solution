package store

import (
	"event-service/internal/model"
	"sync"
)

// Store provides in-memory storage for event idempotency tracking.
//
// LIMITATION: This is a simple in-memory store with no persistence.
// All state will be lost when the service restarts.
// In production, this would need to be backed by a durable data store
// like PostgreSQL, Redis, or similar.
type Store struct {
	mu     sync.RWMutex
	events map[string]*model.Event
}

// New creates a new in-memory store
func New() *Store {
	return &Store{
		events: make(map[string]*model.Event),
	}
}

// Exists checks if an event with the given ID has already been accepted
func (s *Store) Exists(eventID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.events[eventID]
	return exists
}

// Save stores an event with the given status
func (s *Store) Save(event *model.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[event.EventID] = event
}

// MarkProcessed updates the event status to processed
func (s *Store) MarkProcessed(eventID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event, exists := s.events[eventID]; exists {
		event.Status = model.StatusProcessed
	}
}

// GetStatus returns the current status of an event
func (s *Store) GetStatus(eventID string) (model.EventStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if event, exists := s.events[eventID]; exists {
		return event.Status, true
	}
	return "", false
}

// List returns all events in the store
func (s *Store) List() []*model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]*model.Event, 0, len(s.events))
	for _, event := range s.events {
		events = append(events, event)
	}
	return events
}
