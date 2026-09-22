package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultHTTPPort = 8080
)

type HTTPConfig struct {
	Port int
}

func LoadHTTPPort() (HTTPConfig, error) {
	raw := os.Getenv("HTTP_PORT")
	if raw == "" {
		return HTTPConfig{Port: defaultHTTPPort}, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return HTTPConfig{}, fmt.Errorf("HTTP_PORT must be an integer, got %q: %w", raw, err)
	}
	if port < 1 || port > 65535 {
		return HTTPConfig{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535, got %d", port)
	}
	return HTTPConfig{Port: port}, nil
}

type DBConfig struct {
	URL string
}

func LoadDB() (DBConfig, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return DBConfig{}, fmt.Errorf("DATABASE_URL is required")
	}
	return DBConfig{URL: url}, nil
}
