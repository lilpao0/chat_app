package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultWSSendQueueCapacity = 64
	defaultWSMaxMessageBytes   = int64(16 * 1024)
	defaultWSWriteTimeout      = 10 * time.Second
	defaultWSPongTimeout       = 60 * time.Second
	defaultWSPingInterval      = 25 * time.Second
	defaultWSShutdownTimeout   = 10 * time.Second

	maxWSSendQueueCapacity = 10_000
	maxWSMessageBytes      = int64(1024 * 1024)
)

type WebSocketConfig struct {
	CommandQueueCapacity int
	CommandTimeout       time.Duration
	CommandRate          int
	CommandBurst         int
	SendQueueCapacity    int
	MaxMessageBytes      int64
	WriteTimeout         time.Duration
	PongTimeout          time.Duration
	PingInterval         time.Duration
	ShutdownTimeout      time.Duration
}

func LoadWebSocket() (WebSocketConfig, error) {
	queue, err := loadPositiveInt("WS_COMMAND_QUEUE_CAPACITY", 16, 10000)
	if err != nil {
		return WebSocketConfig{}, err
	}
	timeout, err := loadDuration("WS_COMMAND_TIMEOUT", 5*time.Second)
	if err != nil {
		return WebSocketConfig{}, err
	}
	rate, err := loadPositiveInt("WS_COMMAND_RATE", 10, 10000)
	if err != nil {
		return WebSocketConfig{}, err
	}
	burst, err := loadPositiveInt("WS_COMMAND_BURST", 20, 10000)
	if err != nil {
		return WebSocketConfig{}, err
	}
	sendQueueCapacity, err := loadPositiveInt(
		"WS_SEND_QUEUE_CAPACITY",
		defaultWSSendQueueCapacity,
		maxWSSendQueueCapacity,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	maxMessageBytes, err := loadPositiveInt64(
		"WS_MAX_MESSAGE_BYTES",
		defaultWSMaxMessageBytes,
		maxWSMessageBytes,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	writeTimeout, err := loadDuration(
		"WS_WRITE_TIMEOUT",
		defaultWSWriteTimeout,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	pongTimeout, err := loadDuration(
		"WS_PONG_TIMEOUT",
		defaultWSPongTimeout,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	pingInterval, err := loadDuration(
		"WS_PING_INTERVAL",
		defaultWSPingInterval,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	shutdownTimeout, err := loadDuration(
		"WS_SHUTDOWN_TIMEOUT",
		defaultWSShutdownTimeout,
	)
	if err != nil {
		return WebSocketConfig{}, err
	}

	if pingInterval >= pongTimeout {
		return WebSocketConfig{}, errors.New(
			"WS_PING_INTERVAL must be shorter than WS_PONG_TIMEOUT",
		)
	}

	return WebSocketConfig{
		CommandQueueCapacity: queue, CommandTimeout: timeout, CommandRate: rate, CommandBurst: burst,
		SendQueueCapacity: sendQueueCapacity,
		MaxMessageBytes:   maxMessageBytes,
		WriteTimeout:      writeTimeout,
		PongTimeout:       pongTimeout,
		PingInterval:      pingInterval,
		ShutdownTimeout:   shutdownTimeout,
	}, nil
}

func loadPositiveInt(
	name string,
	defaultValue int,
	maximum int,
) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}

	if value < 1 || value > maximum {
		return 0, fmt.Errorf(
			"%s must be between 1 and %d",
			name,
			maximum,
		)
	}

	return value, nil
}

func loadPositiveInt64(
	name string,
	defaultValue int64,
	maximum int64,
) (int64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}

	if value < 1 || value > maximum {
		return 0, fmt.Errorf(
			"%s must be between 1 and %d",
			name,
			maximum,
		)
	}

	return value, nil
}

func loadDuration(
	name string,
	defaultValue time.Duration,
) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value < time.Second {
		return 0, fmt.Errorf(
			"%s must be a duration of at least one second",
			name,
		)
	}

	return value, nil
}
