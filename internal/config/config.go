package config

import (
	"fmt"
	"os"

	"github.com/lpernett/godotenv"
)

type Config struct {
	GeminiApiKey        string
	GeminiModel         string
	GeminiApiUrl        string
	ModerationSecretKey string
	RabbitMQUrl         string
	ApiBaseUrl          string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	secretKey := GetEnv("MODERATION_SERVICE_KEY", GetEnv("MODERATION_SECRET_KEY", ""))
	if secretKey == "" {
		return nil, fmt.Errorf("MODERATION_SERVICE_KEY is required")
	}

	cfg := &Config{
		ModerationSecretKey: secretKey,
		GeminiApiKey:        GetEnv("GEMINI_API_KEY", ""),
		GeminiModel:         GetEnv("GEMINI_MODEL", "gemini-decision-1"),
		GeminiApiUrl:        GetEnv("GEMINI_API_URL", "https://generativelanguage.googleapis.com/v1beta/decisions"),
		ApiBaseUrl:          GetEnv("API_BASE_URL", "http://localhost:8000"),
		RabbitMQUrl:         GetEnv("RABBITMQ_URL", "amqp://pixel:pixel@localhost:5672"),
	}

	return cfg, nil
}

func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
