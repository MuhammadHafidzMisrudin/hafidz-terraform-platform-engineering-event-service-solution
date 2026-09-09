package worker

import (
	"log"
	"event-service/internal/model"
	"event-service/internal/store"
	"time"
)

// Worker processes events asynchronously in the background
type Worker struct {
	queue          chan *model.Event
	store          *store.Store
	processingDelay time.Duration
	running        bool
	done           chan struct{}
}

// New creates a new background worker
func New(store *store.Store, processingDelayMs int) *Worker {
	return &Worker{
		queue:           make(chan *model.Event, 100), // buffered channel
		store:           store,
		processingDelay: time.Duration(processingDelayMs) * time.Millisecond,
		done:            make(chan struct{}),
	}
}

// Start begins processing events from the queue
func (w *Worker) Start() {
	w.running = true
	log.Printf("Worker started with processing delay: %v", w.processingDelay)

	go func() {
		for {
			select {
			case event := <-w.queue:
				w.processEvent(event)
			case <-w.done:
				log.Println("Worker shutting down")
				w.running = false
				return
			}
		}
	}()
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	log.Println("Stopping worker...")
	close(w.done)
	// Drain remaining events in the queue
	for len(w.queue) > 0 {
		event := <-w.queue
		w.processEvent(event)
	}
}

// Enqueue adds an event to the processing queue
func (w *Worker) Enqueue(event *model.Event) {
	w.queue <- event
}

// IsRunning returns whether the worker is currently running
func (w *Worker) IsRunning() bool {
	return w.running
}

// processEvent simulates event processing with a configurable delay
func (w *Worker) processEvent(event *model.Event) {
	log.Printf("Processing event: %s", event.EventID)

	// Simulate work
	time.Sleep(w.processingDelay)

	// Mark as processed
	w.store.MarkProcessed(event.EventID)
	log.Printf("Event processed: %s", event.EventID)
}
