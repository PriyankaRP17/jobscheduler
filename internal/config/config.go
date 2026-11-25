package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort    string
	MongoURI    string
	MongoDB     string
	NatsURL     string
	JWTSecret   string
	WorkerID    string
	LogLevel    string
}

func Load() *Config {
	// Load .env if present (ignored in production)
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort:  getEnv("HTTP_PORT", "8080"),
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:   getEnv("MONGO_DB", "jobscheduler"),
		NatsURL:   getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
		WorkerID:  getEnv("WORKER_ID", hostname()),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("warn: invalid %s=%q, using %d", key, v, fallback)
			return fallback
		}
		return n
	}
	return fallback
}

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "worker-unknown"
	}
	return h
}
