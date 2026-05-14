package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken      string
	DatabasePath          string
	ImageCachePath        string
	VerifyImageCount      int
	VerifyRequiredCorrect int
	VerifyTimeoutSeconds  int
	VerifyMaxRetry        int
	VerifyDistractorCount int
	RejoinCooldownSeconds int
	DefaultLanguage       string
	InitialAdminIDs       []int64

	ModelBaseURL        string
	ModelAPIKey         string
	ModelName           string
	ModelTimeoutSeconds int
}

func Load() *Config {
	return &Config{
		TelegramBotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		DatabasePath:          getEnv("DATABASE_PATH", "./data/bot.db"),
		ImageCachePath:        getEnv("IMAGE_CACHE_PATH", "./images"),
		VerifyImageCount:      getEnvInt("VERIFY_IMAGE_COUNT", 3),
		VerifyRequiredCorrect: getEnvInt("VERIFY_REQUIRED_CORRECT", 2),
		VerifyTimeoutSeconds:  getEnvInt("VERIFY_TIMEOUT_SECONDS", 120),
		VerifyMaxRetry:        getEnvInt("VERIFY_MAX_RETRY", 1),
		VerifyDistractorCount: getEnvInt("VERIFY_DISTRACTOR_COUNT", 3),
		RejoinCooldownSeconds: getEnvInt("REJOIN_COOLDOWN_SECONDS", 300),
		DefaultLanguage:       getEnv("DEFAULT_LANGUAGE", "zh"),
		InitialAdminIDs:       getEnvInt64List("BOT_ADMIN_IDS"),

		ModelBaseURL:        getEnv("MODEL_BASE_URL", ""),
		ModelAPIKey:         getEnv("MODEL_API_KEY", ""),
		ModelName:           getEnv("MODEL_NAME", ""),
		ModelTimeoutSeconds: getEnvInt("MODEL_TIMEOUT_SECONDS", 5),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		log.Printf("Warning: invalid integer value for %s: %s, using default %d", key, value, defaultValue)
	}
	return defaultValue
}

func getEnvInt64List(key string) []int64 {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	var result []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if id, err := strconv.ParseInt(p, 10, 64); err == nil {
			result = append(result, id)
		} else {
			log.Printf("Warning: invalid admin ID in %s: %s", key, p)
		}
	}
	return result
}
