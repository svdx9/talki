package config

import (
	"errors"
	"log/slog"
	"testing"
)

func TestLoad_MissingMistralKey(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	_, err := Load()
	if !errors.Is(err, ErrMissingMistralKey) {
		t.Errorf("want ErrMissingMistralKey, got %v", err)
	}
}

func TestLoad_MissingAnthropicKey(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-key")
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := Load()
	if !errors.Is(err, ErrMissingAnthropicKey) {
		t.Errorf("want ErrMissingAnthropicKey, got %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MistralVoiceID != "female-1" {
		t.Errorf("MistralVoiceID: want female-1, got %s", cfg.MistralVoiceID)
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
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	t.Setenv("PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid PORT")
	}
}

func TestLoad_DebugWSParsing(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")

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
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	t.Setenv("DEBUG_WS", "maybe")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid DEBUG_WS")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
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
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")

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
	t.Setenv("MISTRAL_API_KEY", "test-mistral")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := Load()
	if err == nil {
		t.Error("want error for invalid LOG_LEVEL")
	}
}

func TestConfigRedacted(t *testing.T) {
	t.Parallel()
	cfg := Config{
		MistralAPIKey:   "secret-mistral",
		AnthropicAPIKey: "secret-anthropic",
		MistralVoiceID:  "female-1",
		Port:            8787,
		DebugWS:         false,
		LogLevel:        slog.LevelInfo,
		Env:             "dev",
	}

	redacted := cfg.Redacted()

	if redacted.MistralAPIKey != "set" {
		t.Errorf("MistralAPIKey: want set, got %s", redacted.MistralAPIKey)
	}
	if redacted.AnthropicAPIKey != "set" {
		t.Errorf("AnthropicAPIKey: want set, got %s", redacted.AnthropicAPIKey)
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
