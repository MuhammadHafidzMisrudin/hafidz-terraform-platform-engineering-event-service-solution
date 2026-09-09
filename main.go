package main

import (
	"log"
	"os"
	"os/signal"
	"event-service/internal/app"
	"syscall"
)

func main() {
	log.Println("Event Service starting...")

	// Load configuration
	config := app.LoadConfig()

	// Create application
	application := app.New(config)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := application.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)

	// Graceful shutdown
	application.Shutdown()
	log.Println("Service stopped")
}
