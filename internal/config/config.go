package config

import (
	"fmt"
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	DBUser    string
	DBPass    string
	DBHost    string
	DBPort    string
	DBName    string
	SSLMode   string
	RedisHost string
	RedisPort string
	NatsHost  string
	NatsPort  string
	ApiPort   string
}

func New() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBUser:    os.Getenv("POSTGRES_USER"),
		DBPass:    os.Getenv("POSTGRES_PASSWORD"),
		DBHost:    os.Getenv("POSTGRES_HOST"),
		DBPort:    os.Getenv("POSTGRES_PORT"),
		DBName:    os.Getenv("POSTGRES_DB"),
		SSLMode:   os.Getenv("POSTGRES_SSLMODE"),
		RedisHost: os.Getenv("REDIS_HOST"),
		RedisPort: os.Getenv("REDIS_PORT"),
		NatsHost:  os.Getenv("NATS_HOST"),
		NatsPort:  os.Getenv("NATS_PORT"),
		ApiPort:   os.Getenv("API_PORT"),
	}

	if cfg.DBUser == "" || cfg.DBHost == "" || cfg.DBName == "" || cfg.SSLMode == "" {
		return nil, fmt.Errorf("missing required environment variables for database connection")
	}
	if cfg.RedisHost == "" || cfg.RedisPort == "" {
		return nil, fmt.Errorf("missing required environment variables for redis connection")
	}
	if cfg.NatsHost == "" || cfg.NatsPort == "" {
		return nil, fmt.Errorf("missing required environment variables for nats connection")
	}
	if cfg.ApiPort == "" {
		return nil, fmt.Errorf("missing required environment variables for api connection")
	}

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

func (c *Config) ApiAddr() string {
	return fmt.Sprintf(":%s", c.ApiPort)
}