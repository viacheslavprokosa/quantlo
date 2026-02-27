package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUser         string
	DBPass         string
	DBHost         string
	DBPort         string
	DBName         string
	SSLMode        string
	RedisHost      string
	RedisPort      string
	NatsHost       string
	NatsPort       string
	ApiPort        string
	BusProvider    string
	GRPCHost       string
	GRPCPort       string
	ApiEnabled     string
	BusBufferSize  int
	WorkerProvider string
}

// New loads and validates configuration from environment variables.
// HTTP server is optional: if QANTLO_API_ENABLED != "true", ApiAddr() returns an error
// and the HTTP server simply won't start. The same applies to NATS/gRPC bus providers.
func New() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBUser:         os.Getenv("QANTLO_POSTGRES_USER"),
		DBPass:         os.Getenv("QANTLO_POSTGRES_PASSWORD"),
		DBHost:         os.Getenv("QANTLO_POSTGRES_HOST"),
		DBPort:         os.Getenv("QANTLO_POSTGRES_PORT"),
		DBName:         os.Getenv("QANTLO_POSTGRES_DB"),
		SSLMode:        os.Getenv("QANTLO_POSTGRES_SSLMODE"),
		RedisHost:      os.Getenv("QANTLO_REDIS_HOST"),
		RedisPort:      os.Getenv("QANTLO_REDIS_PORT"),
		NatsHost:       os.Getenv("QANTLO_NATS_HOST"),
		NatsPort:       os.Getenv("QANTLO_NATS_PORT"),
		GRPCHost:       os.Getenv("QANTLO_GRPC_HOST"),
		GRPCPort:       os.Getenv("QANTLO_GRPC_PORT"),
		BusProvider:    os.Getenv("QANTLO_BUS_PROVIDER"),
		ApiPort:        os.Getenv("QANTLO_API_PORT"),
		ApiEnabled:     os.Getenv("QANTLO_API_ENABLED"),
		BusBufferSize:  getEnvInt("QANTLO_BUS_BUFFER_SIZE", 1024),
		WorkerProvider: os.Getenv("QANTLO_WORKER_PROVIDER"),
	}

	// Required: database
	if cfg.DBUser == "" || cfg.DBHost == "" || cfg.DBName == "" || cfg.SSLMode == "" {
		return nil, fmt.Errorf("missing required env for database: QANTLO_POSTGRES_USER/HOST/DB/SSLMODE")
	}

	// Required: redis
	if cfg.RedisHost == "" || cfg.RedisPort == "" {
		return nil, fmt.Errorf("missing required env for redis: QANTLO_REDIS_HOST/PORT")
	}

	// Required: bus provider
	if cfg.BusProvider == "" {
		return nil, fmt.Errorf("missing required env: QANTLO_BUS_PROVIDER (nats|grpc)")
	}
	if cfg.BusProvider != "nats" && cfg.BusProvider != "grpc" {
		return nil, fmt.Errorf("invalid bus provider %q, must be 'nats' or 'grpc'", cfg.BusProvider)
	}

	// Required: worker provider (default to bus provider if empty)
	if cfg.WorkerProvider == "" {
		cfg.WorkerProvider = cfg.BusProvider
	}
	if cfg.WorkerProvider != "nats" && cfg.WorkerProvider != "grpc" {
		return nil, fmt.Errorf("invalid worker provider %q, must be 'nats' or 'grpc'", cfg.WorkerProvider)
	}
	if cfg.BusProvider == "grpc" && (cfg.GRPCHost == "" || cfg.GRPCPort == "") {
		return nil, fmt.Errorf("missing required env for grpc bus: QANTLO_GRPC_HOST/PORT")
	}
	if cfg.BusProvider == "nats" && (cfg.NatsHost == "" || cfg.NatsPort == "") {
		return nil, fmt.Errorf("missing required env for nats bus: QANTLO_NATS_HOST/PORT")
	}

	// Optional: HTTP API — ApiAddr() will return an error if not enabled.
	// Optional: GRPC server — GRPCAddr() will return an error if not configured.

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName, c.SSLMode)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func (c *Config) NatsAddr() string {
	return fmt.Sprintf("nats://%s:%s", c.NatsHost, c.NatsPort)
}

func (c *Config) GRPCAddr() string {
	return fmt.Sprintf("%s:%s", c.GRPCHost, c.GRPCPort)
}

// ApiAddr returns the HTTP listen address if the API is enabled.
// Returns an error if QANTLO_API_ENABLED != "true" — callers should skip starting the HTTP server.
func (c *Config) ApiAddr() (string, error) {
	if c.ApiEnabled == "true" {
		if c.ApiPort == "" {
			return "", fmt.Errorf("QANTLO_API_PORT is required when QANTLO_API_ENABLED=true")
		}
		return ":" + c.ApiPort, nil
	}
	return "", fmt.Errorf("HTTP API is disabled (QANTLO_API_ENABLED != true)")
}

// BusAddr returns the connection address for the configured bus provider.
func (c *Config) BusAddr() string {
	if c.BusProvider == "nats" {
		return c.NatsAddr()
	}
	return c.GRPCAddr()
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	var intVal int
	if _, err := fmt.Sscanf(val, "%d", &intVal); err != nil {
		return defaultVal
	}
	return intVal
}
