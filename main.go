package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lukamindo/rico_parser_go/rico"
)

const (
	interval = 1 * time.Minute // Check rate every 1 minute

)

func main() {

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	channelID := os.Getenv("TELEGRAM_CHANNEL_ID")

	if botToken == "" || channelID == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHANNEL_ID must be set in the environment")
	}

	rc, err := rico.NewRateChecker(botToken, channelID)
	if err != nil {
		log.Fatalf("Failed to create RateChecker: %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigChan
		log.Printf("Received signal: %s, shutting down gracefully...\n", s)
		cancel()
	}()

	// Immediate check on startup
	rc.CheckForRateChange(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, shutting down.")
			return
		case <-ticker.C:
			rc.CheckForRateChange(ctx)
		}
	}
}
