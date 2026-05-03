package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

var (
	ErrMissingMistralKey   = errors.New("missing required MISTRAL_API_KEY")
	ErrMissingAnthropicKey = errors.New("missing required ANTHROPIC_API_KEY")
	ErrInvalidBool         = errors.New("invalid boolean value")
	ErrInvalidLogLevel     = errors.New("invalid log level")
)

type Config struct {
	MistralAPIKey   string
	AnthropicAPIKey string
	MistralVoiceID  string
	Port            int
	DebugWS         bool
	LogLevel        slog.Level
	Env             string
}

func Load() (Config, error) {
	mistralKey := requireEnv("MISTRAL_API_KEY")
	if mistralKey == "" {
		return Config{}, fmt.Errorf("config: mistral_api_key: %w", ErrMissingMistralKey)
	}

	anthropicKey := requireEnv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		return Config{}, fmt.Errorf("config: anthropic_api_key: %w", ErrMissingAnthropicKey)
	}

	mistralVoiceID := getEnvOrDefault("MISTRAL_VOICE_ID", "female-1")

	portStr := getEnvOrDefault("PORT", "8787")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("config: port: %w", err)
	}

	debugWSStr := getEnvOrDefault("DEBUG_WS", "false")
	debugWS, err := parseBool(debugWSStr)
	if err != nil {
		return Config{}, fmt.Errorf("config: debug_ws: %w", err)
	}

	logLevelStr := getEnvOrDefault("LOG_LEVEL", "info")
	logLevel, err := parseLogLevel(logLevelStr)
	if err != nil {
		return Config{}, fmt.Errorf("config: log_level: %w", err)
	}

	env := getEnvOrDefault("ENV", "dev")

	return Config{
		MistralAPIKey:   mistralKey,
		AnthropicAPIKey: anthropicKey,
		MistralVoiceID:  mistralVoiceID,
		Port:            port,
		DebugWS:         debugWS,
		LogLevel:        logLevel,
		Env:             env,
	}, nil
}

func requireEnv(key string) string {
	val, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(val) == "" {
		return ""
	}
	return strings.TrimSpace(val)
}

func getEnvOrDefault(key, defaultVal string) string {
	val, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(val) == "" {
		return defaultVal
	}
	return strings.TrimSpace(val)
}

func parseBool(val string) (bool, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return false, nil
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrInvalidBool, val)
	}
}

func parseLogLevel(val string) (slog.Level, error) {
	val = strings.TrimSpace(val)
	switch strings.ToLower(val) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.Level(0), fmt.Errorf("%w: %q", ErrInvalidLogLevel, val)
	}
}

type ConfigRedacted struct {
	MistralAPIKey   string
	AnthropicAPIKey string
	MistralVoiceID  string
	Port            int
	DebugWS         bool
	LogLevel        string
	Env             string
}

func (c Config) Redacted() ConfigRedacted {
	return ConfigRedacted{
		MistralAPIKey:   "set",
		AnthropicAPIKey: "set",
		MistralVoiceID:  c.MistralVoiceID,
		Port:            c.Port,
		DebugWS:         c.DebugWS,
		LogLevel:        c.LogLevel.String(),
		Env:             c.Env,
	}
}
