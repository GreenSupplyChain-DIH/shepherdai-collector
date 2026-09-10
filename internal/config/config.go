package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	JetsonBaseURL    string
	JetsonAPIKey     string
	JetsonAPIKeyName string
	JetsonEndpoint   string
	PollInterval     time.Duration
	RequestTimeout   time.Duration
	DatabaseURL      string
	HealthAddr       string
	BufferPath       string
	TargetCowIDs     map[int]struct{}
	LogLevel         slog.Level
	BatchSize        int
	BackoffBase      time.Duration
	BackoffMax       time.Duration
	SchemaName       string
	TelemetryTable   string
	AlertsTable      string
	AllowAllCowIDs   bool
}

func Load() (Config, error) {
	cfg := Config{
		JetsonBaseURL:    strings.TrimRight(os.Getenv("JETSON_BASE_URL"), "/"),
		JetsonAPIKey:     os.Getenv("JETSON_API_KEY"),
		JetsonAPIKeyName: defaultString(os.Getenv("JETSON_API_KEY_HEADER"), "x-api-key"),
		JetsonEndpoint:   defaultString(os.Getenv("JETSON_ENDPOINT"), "/api/v1/cows"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		HealthAddr:       defaultString(os.Getenv("HEALTH_ADDR"), ":8081"),
		BufferPath:       defaultString(os.Getenv("BUFFER_PATH"), "./var/spool/pending.jsonl"),
		BatchSize:        defaultInt(os.Getenv("BATCH_SIZE"), 100),
		BackoffBase:      defaultDuration(os.Getenv("BACKOFF_BASE"), 2*time.Second),
		BackoffMax:       defaultDuration(os.Getenv("BACKOFF_MAX"), 30*time.Second),
		SchemaName:       defaultString(os.Getenv("PG_SCHEMA"), "jetson_telemetry"),
		TelemetryTable:   defaultString(os.Getenv("PG_TELEMETRY_TABLE"), "cow_telemetry_history"),
		AlertsTable:      defaultString(os.Getenv("PG_ALERTS_TABLE"), "cow_alert_events"),
		AllowAllCowIDs:   strings.EqualFold(os.Getenv("ALLOW_ALL_COW_IDS"), "true"),
	}

	cfg.PollInterval = defaultDuration(os.Getenv("POLL_INTERVAL"), 2*time.Second)
	cfg.RequestTimeout = defaultDuration(os.Getenv("REQUEST_TIMEOUT"), 10*time.Second)
	cfg.LogLevel = parseLogLevel(defaultString(os.Getenv("LOG_LEVEL"), "INFO"))

	ids, err := parseCowIDs(os.Getenv("TARGET_COW_IDS"))
	if err != nil {
		return Config{}, err
	}
	cfg.TargetCowIDs = ids

	if cfg.JetsonBaseURL == "" {
		return Config{}, errors.New("JETSON_BASE_URL is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if !cfg.AllowAllCowIDs && len(cfg.TargetCowIDs) == 0 {
		return Config{}, errors.New("TARGET_COW_IDS is required unless ALLOW_ALL_COW_IDS=true")
	}

	return cfg, nil
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func defaultDuration(value string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCowIDs(raw string) (map[int]struct{}, error) {
	result := make(map[int]struct{})
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return result, nil
	}

	for _, token := range strings.Split(trimmed, ",") {
		value := strings.TrimSpace(token)
		if value == "" {
			continue
		}
		cowID, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid TARGET_COW_IDS value %q: %w", value, err)
		}
		result[cowID] = struct{}{}
	}

	return result, nil
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
