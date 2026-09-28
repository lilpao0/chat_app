package config

import (
	"strings"
	"testing"
	"time"
)

var webSocketEnvironmentKeys = []string{
	"WS_COMMAND_QUEUE_CAPACITY", "WS_COMMAND_TIMEOUT", "WS_COMMAND_RATE", "WS_COMMAND_BURST",
	"WS_SEND_QUEUE_CAPACITY",
	"WS_MAX_MESSAGE_BYTES",
	"WS_WRITE_TIMEOUT",
	"WS_PONG_TIMEOUT",
	"WS_PING_INTERVAL",
	"WS_SHUTDOWN_TIMEOUT",
}

func clearWebSocketEnvironment(t *testing.T) {
	t.Helper()

	for _, key := range webSocketEnvironmentKeys {
		t.Setenv(key, "")
	}
}

func TestLoadWebSocketDefaults(t *testing.T) {
	clearWebSocketEnvironment(t)

	config, err := LoadWebSocket()
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	if config.SendQueueCapacity != 64 {
		t.Fatalf(
			"send queue capacity = %d, want 64",
			config.SendQueueCapacity,
		)
	}
	if config.CommandQueueCapacity != 16 || config.CommandTimeout != 5*time.Second || config.CommandRate != 10 || config.CommandBurst != 20 {
		t.Fatalf("command defaults: %+v", config)
	}

	if config.MaxMessageBytes != 16384 {
		t.Fatalf(
			"max message bytes = %d, want 16384",
			config.MaxMessageBytes,
		)
	}

	if config.WriteTimeout != 10*time.Second {
		t.Fatalf(
			"write timeout = %v, want 10s",
			config.WriteTimeout,
		)
	}

	if config.PongTimeout != 60*time.Second {
		t.Fatalf(
			"pong timeout = %v, want 60s",
			config.PongTimeout,
		)
	}

	if config.PingInterval != 25*time.Second {
		t.Fatalf(
			"ping interval = %v, want 25s",
			config.PingInterval,
		)
	}

	if config.ShutdownTimeout != 10*time.Second {
		t.Fatalf(
			"shutdown timeout = %v, want 10s",
			config.ShutdownTimeout,
		)
	}
}

func TestCommandConfiguration(t *testing.T) {
	for _, key := range []string{"WS_COMMAND_QUEUE_CAPACITY", "WS_COMMAND_RATE", "WS_COMMAND_BURST", "WS_COMMAND_TIMEOUT"} {
		t.Run(key, func(t *testing.T) {
			clearWebSocketEnvironment(t)
			t.Setenv(key, "0")
			if _, err := LoadWebSocket(); err == nil {
				t.Fatal("zero accepted")
			}
			value := "7"
			if key == "WS_COMMAND_TIMEOUT" {
				value = "7s"
			}
			t.Setenv(key, value)
			cfg, err := LoadWebSocket()
			if err != nil {
				t.Fatal(err)
			}
			switch key {
			case "WS_COMMAND_QUEUE_CAPACITY":
				if cfg.CommandQueueCapacity != 7 {
					t.Fatal(cfg)
				}
			case "WS_COMMAND_RATE":
				if cfg.CommandRate != 7 {
					t.Fatal(cfg)
				}
			case "WS_COMMAND_BURST":
				if cfg.CommandBurst != 7 {
					t.Fatal(cfg)
				}
			case "WS_COMMAND_TIMEOUT":
				if cfg.CommandTimeout != 7*time.Second {
					t.Fatal(cfg)
				}
			}
		})
	}
}

func TestLoadWebSocketOverrides(t *testing.T) {
	clearWebSocketEnvironment(t)

	t.Setenv("WS_SEND_QUEUE_CAPACITY", "128")
	t.Setenv("WS_MAX_MESSAGE_BYTES", "2048")
	t.Setenv("WS_WRITE_TIMEOUT", "5s")
	t.Setenv("WS_PONG_TIMEOUT", "45s")
	t.Setenv("WS_PING_INTERVAL", "15s")
	t.Setenv("WS_SHUTDOWN_TIMEOUT", "20s")

	config, err := LoadWebSocket()
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}

	if config.SendQueueCapacity != 128 ||
		config.MaxMessageBytes != 2048 ||
		config.WriteTimeout != 5*time.Second ||
		config.PongTimeout != 45*time.Second ||
		config.PingInterval != 15*time.Second ||
		config.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestLoadWebSocketRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		errorText string
	}{
		{
			name:      "queue is not an integer",
			key:       "WS_SEND_QUEUE_CAPACITY",
			value:     "many",
			errorText: "WS_SEND_QUEUE_CAPACITY",
		},
		{
			name:      "queue is zero",
			key:       "WS_SEND_QUEUE_CAPACITY",
			value:     "0",
			errorText: "WS_SEND_QUEUE_CAPACITY",
		},
		{
			name:      "queue exceeds maximum",
			key:       "WS_SEND_QUEUE_CAPACITY",
			value:     "10001",
			errorText: "WS_SEND_QUEUE_CAPACITY",
		},
		{
			name:      "message size is negative",
			key:       "WS_MAX_MESSAGE_BYTES",
			value:     "-1",
			errorText: "WS_MAX_MESSAGE_BYTES",
		},
		{
			name:      "message size exceeds maximum",
			key:       "WS_MAX_MESSAGE_BYTES",
			value:     "1048577",
			errorText: "WS_MAX_MESSAGE_BYTES",
		},
		{
			name:      "write timeout is invalid",
			key:       "WS_WRITE_TIMEOUT",
			value:     "soon",
			errorText: "WS_WRITE_TIMEOUT",
		},
		{
			name:      "write timeout is too short",
			key:       "WS_WRITE_TIMEOUT",
			value:     "500ms",
			errorText: "WS_WRITE_TIMEOUT",
		},
		{
			name:      "pong timeout is zero",
			key:       "WS_PONG_TIMEOUT",
			value:     "0s",
			errorText: "WS_PONG_TIMEOUT",
		},
		{
			name:      "ping interval is negative",
			key:       "WS_PING_INTERVAL",
			value:     "-1s",
			errorText: "WS_PING_INTERVAL",
		},
		{
			name:      "shutdown timeout is invalid",
			key:       "WS_SHUTDOWN_TIMEOUT",
			value:     "invalid",
			errorText: "WS_SHUTDOWN_TIMEOUT",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearWebSocketEnvironment(t)
			t.Setenv(test.key, test.value)

			_, err := LoadWebSocket()
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), test.errorText) {
				t.Fatalf(
					"error = %q, want it to contain %q",
					err,
					test.errorText,
				)
			}
		})
	}
}

func TestLoadWebSocketRejectsInvalidIntervalRelationship(
	t *testing.T,
) {
	clearWebSocketEnvironment(t)

	t.Setenv("WS_PONG_TIMEOUT", "30s")
	t.Setenv("WS_PING_INTERVAL", "30s")

	_, err := LoadWebSocket()
	if err == nil {
		t.Fatal("ping interval equal to pong timeout was accepted")
	}

	if !strings.Contains(
		err.Error(),
		"WS_PING_INTERVAL must be shorter than WS_PONG_TIMEOUT",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}
