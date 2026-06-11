package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                 string
	HTTPAddr               string
	PublicBaseURL          string
	WebOrigin              string
	PostgresDSN            string
	RedisAddr              string
	RedisPassword          string
	RedisDB                int
	KafkaBrokers           []string
	KafkaClickTopic        string
	KafkaConsumerGroup     string
	RateLimitRequests      int
	RateLimitWindow        time.Duration
	ShortCodeLength        int
	GracefulShutdownPeriod time.Duration
}

func Load() (Config, error) {
	windowSeconds, err := envInt("RATE_LIMIT_WINDOW_SECONDS", 60)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                 envString("APP_ENV", "development"),
		HTTPAddr:               envString("HTTP_ADDR", ":8080"),
		PublicBaseURL:          strings.TrimRight(envString("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		WebOrigin:              envString("WEB_ORIGIN", "http://localhost:5173"),
		PostgresDSN:            envString("POSTGRES_DSN", "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"),
		RedisAddr:              envString("REDIS_ADDR", "localhost:6379"),
		RedisPassword:          envString("REDIS_PASSWORD", ""),
		KafkaBrokers:           splitCSV(envString("KAFKA_BROKERS", "localhost:9092")),
		KafkaClickTopic:        envString("KAFKA_CLICK_TOPIC", "click-events"),
		KafkaConsumerGroup:     envString("KAFKA_CONSUMER_GROUP", "analytics-worker"),
		RateLimitWindow:        time.Duration(windowSeconds) * time.Second,
		GracefulShutdownPeriod: 10 * time.Second,
	}

	if cfg.RedisDB, err = envInt("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitRequests, err = envInt("RATE_LIMIT_REQUESTS", 120); err != nil {
		return Config{}, err
	}
	if cfg.ShortCodeLength, err = envInt("SHORT_CODE_LENGTH", 7); err != nil {
		return Config{}, err
	}
	if len(cfg.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}

	return cfg, nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func splitCSV(raw string) []string {
	var values []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}
