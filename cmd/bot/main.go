package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/szres/ing-recaptcha/internal/bot"
	"github.com/szres/ing-recaptcha/internal/config"
	"github.com/szres/ing-recaptcha/internal/database"
)

func main() {
	cfg := config.Load()

	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	log.Printf("Starting Ingress reCAPTCHA Bot...")
	log.Printf("Config: ImageCount=%d, RequiredCorrect=%d, Timeout=%ds, MaxRetry=%d",
		cfg.VerifyImageCount, cfg.VerifyRequiredCorrect, cfg.VerifyTimeoutSeconds, cfg.VerifyMaxRetry)

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}
	if err := os.MkdirAll(cfg.ImageCachePath, 0755); err != nil {
		log.Fatalf("Failed to create image cache directory: %v", err)
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(""); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize admins from config
	for _, adminID := range cfg.InitialAdminIDs {
		if err := db.AddAdmin(adminID); err != nil {
			log.Printf("Warning: failed to add initial admin %d: %v", adminID, err)
		} else {
			log.Printf("Added initial admin: %d", adminID)
		}
	}

	// Create bot
	b, err := bot.New(cfg, db)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal...")
		b.Stop()
	}()

	// Start bot (blocks until stopped)
	if err := b.Start(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}

	log.Println("Bot shutdown complete")
}
