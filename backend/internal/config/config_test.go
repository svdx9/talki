package config

import (
	"errors"
	"log/slog"
	"testing"
)

// setAnthropicProvider is a helper that configures the minimal env vars for
// the anthropic provider so tests that focus on other fields don't repeat it.
func setAnthropicProvider(t *testing.T) {
	t.Helper()
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
}

func TestLoad_MissingMistralKey(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "")

	_, err := Load()
	if !errors.Is(err, ErrMissingMistralKey) {
		t.Errorf("want ErrMissingMistralKey, got %v", err)
	}
}

func TestLoad_MissingLLMProvider(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_PROVIDER", "")

	_, err := Load()
	if !errors.Is(err, ErrMissingLLMProvider) {
		t.Errorf("want ErrMissingLLMProvider, got %v", err)
	}
}

func TestLoad_InvalidLLMProvider(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_PROVIDER", "groq")

	_, err := Load()
	if !errors.Is(err, ErrInvalidLLMProvider) {
		t.Errorf("want ErrInvalidLLMProvider, got %v", err)
	}
}

func TestLoad_MissingAnthropicKey(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-key")
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := Load()
	if !errors.Is(err, ErrMissingAnthropicKey) {
		t.Errorf("want ErrMissingAnthropicKey, got %v", err)
	}
}

func TestLoad_MissingOpenCodeKey(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-key")
	t.Setenv("LLM_PROVIDER", "opencode")
	t.Setenv("OPENCODE_API_KEY", "")

	_, err := Load()
	if !errors.Is(err, ErrMissingOpenCodeKey) {
		t.Errorf("want ErrMissingOpenCodeKey, got %v", err)
	}
}

func TestLoad_AnthropicProvider(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMProvider != ProviderAnthropic {
		t.Errorf("LLMProvider: want %s, got %s", ProviderAnthropic, cfg.LLMProvider)
	}
	if cfg.LLMAPIKey != "sk-ant-test" {
		t.Errorf("LLMAPIKey: want sk-ant-test, got %s", cfg.LLMAPIKey)
	}
	if cfg.LLMBaseURL != anthropicBaseURL {
		t.Errorf("LLMBaseURL: want %s, got %s", anthropicBaseURL, cfg.LLMBaseURL)
	}
}

func TestLoad_OpenCodeProvider(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_PROVIDER", "opencode")
	t.Setenv("OPENCODE_API_KEY", "oc-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMProvider != ProviderOpenCode {
		t.Errorf("LLMProvider: want %s, got %s", ProviderOpenCode, cfg.LLMProvider)
	}
	if cfg.LLMAPIKey != "oc-test" {
		t.Errorf("LLMAPIKey: want oc-test, got %s", cfg.LLMAPIKey)
	}
	if cfg.LLMBaseURL != openCodeBaseURL {
		t.Errorf("LLMBaseURL: want %s, got %s", openCodeBaseURL, cfg.LLMBaseURL)
	}
}

func TestLoad_LLMModelDefault(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_MODEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMModel != DefaultLLMModel {
		t.Errorf("LLMModel: want %s, got %s", DefaultLLMModel, cfg.LLMModel)
	}
}

func TestLoad_LLMModelOverride(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LLM_MODEL", "claude-opus-4-7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMModel != "claude-opus-4-7" {
		t.Errorf("LLMModel: want claude-opus-4-7, got %s", cfg.LLMModel)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MistralVoiceID != "fr_marie_neutral" {
		t.Errorf("MistralVoiceID: want fr_marie_neutral, got %s", cfg.MistralVoiceID)
	}
	if cfg.Port != 8787 {
		t.Errorf("Port: want 8787, got %d", cfg.Port)
	}
	if cfg.DebugWS != false {
		t.Errorf("DebugWS: want false, got %v", cfg.DebugWS)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel: want LevelInfo, got %v", cfg.LogLevel)
	}
	if cfg.Env != "dev" {
		t.Errorf("Env: want dev, got %s", cfg.Env)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid PORT")
	}
}

func TestLoad_DebugWSParsing(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true lowercase", "true", true},
		{"True capital", "True", true},
		{"TRUE all caps", "TRUE", true},
		{"1", "1", true},
		{"yes lowercase", "yes", true},
		{"Yes capital", "Yes", true},
		{"false lowercase", "false", false},
		{"False capital", "False", false},
		{"FALSE all caps", "FALSE", false},
		{"0", "0", false},
		{"no lowercase", "no", false},
		{"No capital", "No", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEBUG_WS", tt.value)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.DebugWS != tt.want {
				t.Errorf("DebugWS: want %v, got %v", tt.want, cfg.DebugWS)
			}
		})
	}
}

func TestLoad_InvalidDebugWS(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("DEBUG_WS", "maybe")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid DEBUG_WS")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("MISTRAL_VOICE_ID", "male-2")
	t.Setenv("PORT", "3000")
	t.Setenv("DEBUG_WS", "true")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ENV", "prod")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MistralVoiceID != "male-2" {
		t.Errorf("MistralVoiceID: want male-2, got %s", cfg.MistralVoiceID)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port: want 3000, got %d", cfg.Port)
	}
	if cfg.DebugWS != true {
		t.Errorf("DebugWS: want true, got %v", cfg.DebugWS)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel: want LevelDebug, got %v", cfg.LogLevel)
	}
	if cfg.Env != "prod" {
		t.Errorf("Env: want prod, got %s", cfg.Env)
	}
}

func TestLoad_LogLevel(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")

	tests := []struct {
		value string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tt.value)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.LogLevel != tt.want {
				t.Errorf("LogLevel: want %v, got %v", tt.want, cfg.LogLevel)
			}
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	setAnthropicProvider(t)
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid LOG_LEVEL")
	}
}

func TestConfigRedacted(t *testing.T) {
	t.Parallel()
	cfg := Config{
		MistralAPIKey:  "secret-mistral",
		LLMProvider:    ProviderAnthropic,
		LLMAPIKey:      "secret-llm",
		LLMBaseURL:     anthropicBaseURL,
		LLMModel:       DefaultLLMModel,
		MistralVoiceID: "female-1",
		Port:           8787,
		DebugWS:        false,
		LogLevel:       slog.LevelInfo,
		Env:            "dev",
	}

	redacted := cfg.Redacted()

	if redacted.MistralAPIKey != "set" {
		t.Errorf("MistralAPIKey: want set, got %s", redacted.MistralAPIKey)
	}
	if redacted.LLMAPIKey != "set" {
		t.Errorf("LLMAPIKey: want set, got %s", redacted.LLMAPIKey)
	}
	if redacted.LLMProvider != ProviderAnthropic {
		t.Errorf("LLMProvider: want %s, got %s", ProviderAnthropic, redacted.LLMProvider)
	}
	if redacted.LLMBaseURL != anthropicBaseURL {
		t.Errorf("LLMBaseURL: want %s, got %s", anthropicBaseURL, redacted.LLMBaseURL)
	}
	if redacted.LLMModel != DefaultLLMModel {
		t.Errorf("LLMModel: want %s, got %s", DefaultLLMModel, redacted.LLMModel)
	}
	if redacted.MistralVoiceID != "female-1" {
		t.Errorf("MistralVoiceID: want female-1, got %s", redacted.MistralVoiceID)
	}
	if redacted.Port != 8787 {
		t.Errorf("Port: want 8787, got %d", redacted.Port)
	}
	if redacted.DebugWS != false {
		t.Errorf("DebugWS: want false, got %v", redacted.DebugWS)
	}
	if redacted.LogLevel != "INFO" {
		t.Errorf("LogLevel: want INFO, got %s", redacted.LogLevel)
	}
	if redacted.Env != "dev" {
		t.Errorf("Env: want dev, got %s", redacted.Env)
	}
}
