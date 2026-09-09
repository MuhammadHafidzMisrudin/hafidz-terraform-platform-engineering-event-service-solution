package app

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"event-service/internal/model"
	"event-service/internal/store"
	"event-service/internal/worker"
	"time"
)

// Config holds the application configuration
type Config struct {
	Port              string
	Env               string
	ProcessingDelayMs int
}

// App represents the HTTP application
type App struct {
	config    Config
	store     *store.Store
	worker    *worker.Worker
	startTime time.Time
	server    *http.Server
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() Config {
	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "dev")
	processingDelayMs := getEnvAsInt("PROCESSING_DELAY_MS", 1000)

	return Config{
		Port:              port,
		Env:               env,
		ProcessingDelayMs: processingDelayMs,
	}
}

// New creates a new application instance
func New(config Config) *App {
	st := store.New()
	wkr := worker.New(st, config.ProcessingDelayMs)

	return &App{
		config:    config,
		store:     st,
		worker:    wkr,
		startTime: time.Now(),
	}
}

// Start starts the HTTP server and background worker
func (a *App) Start() error {
	a.worker.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("/events", a.handleEvents)
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/ready", a.handleReady)
	mux.HandleFunc("/", a.handleFrontend)

	a.server = &http.Server{
		Addr:    ":" + a.config.Port,
		Handler: mux,
	}

	log.Printf("Starting server on port %s (env: %s)", a.config.Port, a.config.Env)
	return a.server.ListenAndServe()
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown() {
	log.Println("Shutting down application...")
	a.worker.Stop()
	if a.server != nil {
		a.server.Close()
	}
}

// handleEvents handles POST /events (create) and GET /events (list)
func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// List all events
		events := a.store.List()
		response := make([]model.EventResponse, len(events))
		for i, event := range events {
			response[i] = model.EventResponse{
				EventID: event.EventID,
				Payload: event.Payload,
				Status:  event.Status,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.EventID == "" {
		http.Error(w, "event_id is required", http.StatusBadRequest)
		return
	}

	// Check for idempotency
	if a.store.Exists(req.EventID) {
		log.Printf("Event already exists: %s", req.EventID)
		w.WriteHeader(http.StatusConflict)
		return
	}

	// Create and save event
	event := &model.Event{
		EventID: req.EventID,
		Payload: req.Payload,
		Status:  model.StatusAccepted,
	}
	a.store.Save(event)

	// Enqueue for background processing
	a.worker.Enqueue(event)

	log.Printf("Event accepted: %s", req.EventID)
	w.WriteHeader(http.StatusAccepted)
}

// handleHealth handles GET /health
func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(a.startTime).String()
	resp := model.HealthResponse{
		Status: "ok",
		Uptime: uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleReady handles GET /ready
func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !a.worker.IsRunning() {
		resp := model.ReadyResponse{
			Status: "not ready",
			Ready:  false,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := model.ReadyResponse{
		Status: "ready",
		Ready:  true,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Invalid value for %s: %s, using default: %d", key, valueStr, defaultValue)
		return defaultValue
	}
	return value
}

// GetServer returns the underlying HTTP server (useful for testing or custom shutdown)
func (a *App) GetServer() *http.Server {
	return a.server
}

// Worker returns the worker instance (useful for testing)
func (a *App) Worker() *worker.Worker {
	return a.worker
}

// Store returns the store instance (useful for testing)
func (a *App) Store() *store.Store {
	return a.store
}

// handleFrontend serves the minimal HTML frontend
func (a *App) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(frontendHTML))
}

const frontendHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Event Service Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
            color: #333;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        header {
            background: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            margin-bottom: 20px;
        }

        h1 {
            color: #667eea;
            margin-bottom: 10px;
        }

        .status-bar {
            display: flex;
            gap: 20px;
            margin-top: 15px;
            flex-wrap: wrap;
        }

        .status-item {
            padding: 10px 15px;
            background: #f7fafc;
            border-radius: 5px;
            border-left: 3px solid #667eea;
        }

        .status-label {
            font-size: 12px;
            color: #718096;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .status-value {
            font-size: 18px;
            font-weight: 600;
            margin-top: 5px;
        }

        .status-ready {
            color: #48bb78;
        }

        .status-not-ready {
            color: #f56565;
        }

        .grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 20px;
            margin-bottom: 20px;
        }

        @media (max-width: 768px) {
            .grid {
                grid-template-columns: 1fr;
            }
        }

        .card {
            background: white;
            padding: 25px;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }

        h2 {
            color: #2d3748;
            margin-bottom: 20px;
            font-size: 20px;
        }

        .form-group {
            margin-bottom: 15px;
        }

        label {
            display: block;
            margin-bottom: 5px;
            font-weight: 500;
            color: #4a5568;
        }

        input, textarea {
            width: 100%;
            padding: 10px;
            border: 2px solid #e2e8f0;
            border-radius: 5px;
            font-size: 14px;
            font-family: inherit;
            transition: border-color 0.2s;
        }

        input:focus, textarea:focus {
            outline: none;
            border-color: #667eea;
        }

        textarea {
            resize: vertical;
            min-height: 100px;
            font-family: 'Monaco', 'Courier New', monospace;
        }

        button {
            background: #667eea;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 5px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.2s;
            width: 100%;
        }

        button:hover {
            background: #5568d3;
        }

        button:disabled {
            background: #cbd5e0;
            cursor: not-allowed;
        }

        .event-list {
            max-height: 500px;
            overflow-y: auto;
        }

        .event-item {
            padding: 15px;
            border: 2px solid #e2e8f0;
            border-radius: 5px;
            margin-bottom: 10px;
            transition: border-color 0.2s;
        }

        .event-item:hover {
            border-color: #cbd5e0;
        }

        .event-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }

        .event-id {
            font-weight: 600;
            color: #2d3748;
            font-family: 'Monaco', 'Courier New', monospace;
        }

        .event-status {
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
        }

        .status-accepted {
            background: #fef5e7;
            color: #d97706;
        }

        .status-processed {
            background: #d1fae5;
            color: #059669;
        }

        .event-payload {
            background: #f7fafc;
            padding: 10px;
            border-radius: 5px;
            font-family: 'Monaco', 'Courier New', monospace;
            font-size: 12px;
            overflow-x: auto;
        }

        .message {
            padding: 12px;
            border-radius: 5px;
            margin-bottom: 15px;
            display: none;
        }

        .message.show {
            display: block;
        }

        .message.success {
            background: #d1fae5;
            color: #059669;
            border-left: 4px solid #059669;
        }

        .message.error {
            background: #fee2e2;
            color: #dc2626;
            border-left: 4px solid #dc2626;
        }

        .message.warning {
            background: #fef5e7;
            color: #d97706;
            border-left: 4px solid #d97706;
        }

        .empty-state {
            text-align: center;
            padding: 40px;
            color: #a0aec0;
        }

        .refresh-btn {
            background: #48bb78;
            margin-top: 10px;
        }

        .refresh-btn:hover {
            background: #38a169;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Event Service Dashboard</h1>
            <p style="color: #718096; margin-top: 5px;">Monitor and manage events in real-time</p>
            <div class="status-bar">
                <div class="status-item">
                    <div class="status-label">Service Status</div>
                    <div class="status-value" id="service-status">Loading...</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Worker Status</div>
                    <div class="status-value" id="worker-status">Loading...</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Uptime</div>
                    <div class="status-value" id="uptime">-</div>
                </div>
                <div class="status-item">
                    <div class="status-label">Total Events</div>
                    <div class="status-value" id="total-events">0</div>
                </div>
            </div>
        </header>

        <div class="grid">
            <div class="card">
                <h2>Submit New Event</h2>
                <div id="submit-message" class="message"></div>
                <form id="event-form">
                    <div class="form-group">
                        <label for="event-id">Event ID</label>
                        <input type="text" id="event-id" placeholder="evt_123" required>
                    </div>
                    <div class="form-group">
                        <label for="payload">Payload (JSON)</label>
                        <textarea id="payload" placeholder='{"key": "value"}'>{}</textarea>
                    </div>
                    <button type="submit">Submit Event</button>
                </form>
            </div>

            <div class="card">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
                    <h2 style="margin: 0;">Events</h2>
                    <button class="refresh-btn" onclick="loadEvents()" style="width: auto; padding: 8px 16px; margin: 0;">Refresh</button>
                </div>
                <div id="events-list" class="event-list">
                    <div class="empty-state">Loading events...</div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let autoRefresh = null;

        // Load service health
        async function loadHealth() {
            try {
                const response = await fetch('/health');
                const data = await response.json();
                document.getElementById('service-status').textContent = data.status;
                document.getElementById('uptime').textContent = data.uptime;
            } catch (error) {
                document.getElementById('service-status').textContent = 'error';
                document.getElementById('uptime').textContent = '-';
            }
        }

        // Load worker readiness
        async function loadReady() {
            try {
                const response = await fetch('/ready');
                const data = await response.json();
                const statusEl = document.getElementById('worker-status');
                statusEl.textContent = data.ready ? 'ready' : 'not ready';
                statusEl.className = 'status-value ' + (data.ready ? 'status-ready' : 'status-not-ready');
            } catch (error) {
                const statusEl = document.getElementById('worker-status');
                statusEl.textContent = 'error';
                statusEl.className = 'status-value status-not-ready';
            }
        }

        // Load events
        async function loadEvents() {
            try {
                const response = await fetch('/events');
                const events = await response.json();

                document.getElementById('total-events').textContent = events.length;

                const eventsListEl = document.getElementById('events-list');

                if (events.length === 0) {
                    eventsListEl.innerHTML = '<div class="empty-state">No events yet. Submit an event to get started!</div>';
                    return;
                }

                eventsListEl.innerHTML = events.map(event =>
                    '<div class="event-item">' +
                        '<div class="event-header">' +
                            '<span class="event-id">' + escapeHtml(event.event_id) + '</span>' +
                            '<span class="event-status status-' + event.status + '">' + event.status + '</span>' +
                        '</div>' +
                        '<div class="event-payload">' + formatJSON(event.payload) + '</div>' +
                    '</div>'
                ).join('');
            } catch (error) {
                console.error('Failed to load events:', error);
                document.getElementById('events-list').innerHTML = '<div class="empty-state">Failed to load events</div>';
            }
        }

        // Handle form submission
        document.getElementById('event-form').addEventListener('submit', async (e) => {
            e.preventDefault();

            const messageEl = document.getElementById('submit-message');
            const eventId = document.getElementById('event-id').value;
            const payloadText = document.getElementById('payload').value;

            let payload;
            try {
                payload = JSON.parse(payloadText);
            } catch (error) {
                showMessage('error', 'Invalid JSON in payload');
                return;
            }

            try {
                const response = await fetch('/events', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        event_id: eventId,
                        payload: payload
                    })
                });

                if (response.status === 202) {
                    showMessage('success', 'Event accepted and queued for processing!');
                    document.getElementById('event-id').value = '';
                    document.getElementById('payload').value = '{}';
                    setTimeout(loadEvents, 100);
                } else if (response.status === 409) {
                    showMessage('warning', 'Event already exists (duplicate event_id)');
                } else {
                    const text = await response.text();
                    showMessage('error', 'Failed to submit event: ' + text);
                }
            } catch (error) {
                showMessage('error', 'Network error: ' + error.message);
            }
        });

        function showMessage(type, text) {
            const messageEl = document.getElementById('submit-message');
            messageEl.className = 'message show ' + type;
            messageEl.textContent = text;
            setTimeout(() => {
                messageEl.className = 'message';
            }, 5000);
        }

        function formatJSON(obj) {
            if (typeof obj === 'string') {
                try {
                    obj = JSON.parse(obj);
                } catch (e) {
                    return escapeHtml(obj);
                }
            }
            return escapeHtml(JSON.stringify(obj, null, 2));
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Initial load
        loadHealth();
        loadReady();
        loadEvents();

        // Auto-refresh every 2 seconds
        setInterval(() => {
            loadHealth();
            loadReady();
            loadEvents();
        }, 2000);
    </script>
</body>
</html>
`
