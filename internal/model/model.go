package model

import "encoding/json"

// EventRequest represents the incoming POST /events request body
type EventRequest struct {
	EventID string          `json:"event_id"`
	Payload json.RawMessage `json:"payload"`
}

// EventStatus represents the processing state of an event
type EventStatus string

const (
	StatusAccepted  EventStatus = "accepted"
	StatusProcessed EventStatus = "processed"
)

// Event represents an event in the system
type Event struct {
	EventID string
	Payload json.RawMessage
	Status  EventStatus
}

// HealthResponse is returned by GET /health
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// ReadyResponse is returned by GET /ready
type ReadyResponse struct {
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

// EventResponse is returned when listing events
type EventResponse struct {
	EventID string          `json:"event_id"`
	Payload json.RawMessage `json:"payload"`
	Status  EventStatus     `json:"status"`
}
