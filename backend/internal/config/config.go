package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const (
	ProviderAnthropic = "anthropic"
	ProviderOpenCode  = "opencode"

	anthropicBaseURL = "https://api.anthropic.com/v1/messages"
	openCodeBaseURL  = "https://opencode.ai/zen/v1/messages"

	DefaultLLMModel = "claude-sonnet-4-6"
)

var (
	ErrMissingMistralKey   = errors.New("missing required MISTRAL_API_KEY")
	ErrMissingAnthropicKey = errors.New("missing required ANTHROPIC_API_KEY")
	ErrMissingOpenCodeKey  = errors.New("missing required OPENCODE_API_KEY")
	ErrMissingLLMProvider  = errors.New("missing required LLM_PROVIDER")
	ErrInvalidLLMProvider  = errors.New("invalid LLM_PROVIDER: must be anthropic or opencode")
	ErrInvalidBool         = errors.New("invalid boolean value")
	ErrInvalidLogLevel     = errors.New("invalid log level")
)

type Config struct {
	MistralAPIKey  string
	LLMProvider    string
	LLMAPIKey      string
	LLMBaseURL     string
	LLMModel       string
	MistralVoiceID string
	Port           int
	DebugWS        bool
	LogLevel       slog.Level
	Env            string
}

func Load() (Config, error) {
	mistralKey := requireEnv("MISTRAL_API_KEY")
	if mistralKey == "" {
		return Config{}, fmt.Errorf("config: mistral_api_key: %w", ErrMissingMistralKey)
	}

	provider := requireEnv("LLM_PROVIDER")
	if provider == "" {
		return Config{}, fmt.Errorf("config: llm_provider: %w", ErrMissingLLMProvider)
	}

	var llmAPIKey, llmBaseURL string
	switch provider {
	case ProviderAnthropic:
		llmAPIKey = requireEnv("ANTHROPIC_API_KEY")
		if llmAPIKey == "" {
			return Config{}, fmt.Errorf("config: anthropic_api_key: %w", ErrMissingAnthropicKey)
		}
		llmBaseURL = anthropicBaseURL
	case ProviderOpenCode:
		llmAPIKey = requireEnv("OPENCODE_API_KEY")
		if llmAPIKey == "" {
			return Config{}, fmt.Errorf("config: opencode_api_key: %w", ErrMissingOpenCodeKey)
		}
		llmBaseURL = openCodeBaseURL
	default:
		return Config{}, fmt.Errorf("config: llm_provider %q: %w", provider, ErrInvalidLLMProvider)
	}

	llmModel := getEnvOrDefault("LLM_MODEL", DefaultLLMModel)

	mistralVoiceID := getEnvOrDefault("MISTRAL_VOICE_ID", "fr_marie_neutral")

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
		MistralAPIKey:  mistralKey,
		LLMProvider:    provider,
		LLMAPIKey:      llmAPIKey,
		LLMBaseURL:     llmBaseURL,
		LLMModel:       llmModel,
		MistralVoiceID: mistralVoiceID,
		Port:           port,
		DebugWS:        debugWS,
		LogLevel:       logLevel,
		Env:            env,
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
	MistralAPIKey  string
	LLMProvider    string
	LLMAPIKey      string
	LLMBaseURL     string
	LLMModel       string
	MistralVoiceID string
	Port           int
	DebugWS        bool
	LogLevel       string
	Env            string
}

func (c Config) Redacted() ConfigRedacted {
	return ConfigRedacted{
		MistralAPIKey:  "set",
		LLMProvider:    c.LLMProvider,
		LLMAPIKey:      "set",
		LLMBaseURL:     c.LLMBaseURL,
		LLMModel:       c.LLMModel,
		MistralVoiceID: c.MistralVoiceID,
		Port:           c.Port,
		DebugWS:        c.DebugWS,
		LogLevel:       c.LogLevel.String(),
		Env:            c.Env,
	}
}
