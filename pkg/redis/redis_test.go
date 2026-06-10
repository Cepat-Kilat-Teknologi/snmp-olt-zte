package redis

import (
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
)

func TestNewRedisClient_FromConfig(t *testing.T) {
	// Clear environment variables to ensure we use config
	t.Setenv("APP_ENV", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("REDIS_MIN_IDLE_CONNECTIONS", "")
	t.Setenv("REDIS_POOL_SIZE", "")
	t.Setenv("REDIS_POOL_TIMEOUT", "")

	cfg := &config.Config{
		RedisCfg: config.RedisConfig{
			Host:               "localhost",
			Port:               "6379",
			Password:           "testpass",
			DB:                 1,
			MinIdleConnections: 10,
			PoolSize:           100,
			PoolTimeout:        30,
		},
	}

	client := NewRedisClient(cfg)

	if client == nil {
		t.Error("Expected non-nil Redis client")
	}

	opts := client.Options()

	expectedAddr := "localhost:6379"
	if opts.Addr != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, opts.Addr)
	}

	if opts.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got %s", opts.Password)
	}

	if opts.DB != 1 {
		t.Errorf("Expected DB 1, got %d", opts.DB)
	}

	if opts.MinIdleConns != 10 {
		t.Errorf("Expected MinIdleConns 10, got %d", opts.MinIdleConns)
	}

	if opts.PoolSize != 100 {
		t.Errorf("Expected PoolSize 100, got %d", opts.PoolSize)
	}
}

func TestNewRedisClient_FromEnvironment_Development(t *testing.T) {
	// Set environment variables
	t.Setenv("APP_ENV", "development")
	t.Setenv("REDIS_HOST", "redis-dev")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "devpass")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_MIN_IDLE_CONNECTIONS", "20")
	t.Setenv("REDIS_POOL_SIZE", "200")
	t.Setenv("REDIS_POOL_TIMEOUT", "60")

	cfg := &config.Config{
		RedisCfg: config.RedisConfig{
			Host: "should-be-ignored",
			Port: "9999",
		},
	}

	client := NewRedisClient(cfg)

	if client == nil {
		t.Error("Expected non-nil Redis client")
	}

	opts := client.Options()

	expectedAddr := "redis-dev:6380"
	if opts.Addr != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, opts.Addr)
	}

	if opts.Password != "devpass" {
		t.Errorf("Expected password 'devpass', got %s", opts.Password)
	}

	if opts.DB != 2 {
		t.Errorf("Expected DB 2, got %d", opts.DB)
	}

	if opts.MinIdleConns != 20 {
		t.Errorf("Expected MinIdleConns 20, got %d", opts.MinIdleConns)
	}

	if opts.PoolSize != 200 {
		t.Errorf("Expected PoolSize 200, got %d", opts.PoolSize)
	}
}

func TestNewRedisClient_FromEnvironment_Production(t *testing.T) {
	// Set environment variables for production
	t.Setenv("APP_ENV", "production")
	t.Setenv("REDIS_HOST", "redis-prod")
	t.Setenv("REDIS_PORT", "6381")
	t.Setenv("REDIS_PASSWORD", "prodpass")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_MIN_IDLE_CONNECTIONS", "30")
	t.Setenv("REDIS_POOL_SIZE", "300")
	t.Setenv("REDIS_POOL_TIMEOUT", "90")

	cfg := &config.Config{
		RedisCfg: config.RedisConfig{
			Host: "should-be-ignored",
			Port: "9999",
		},
	}

	client := NewRedisClient(cfg)

	if client == nil {
		t.Error("Expected non-nil Redis client")
	}

	opts := client.Options()

	expectedAddr := "redis-prod:6381"
	if opts.Addr != expectedAddr {
		t.Errorf("Expected address %s, got %s", expectedAddr, opts.Addr)
	}

	if opts.Password != "prodpass" {
		t.Errorf("Expected password 'prodpass', got %s", opts.Password)
	}

	if opts.DB != 3 {
		t.Errorf("Expected DB 3, got %d", opts.DB)
	}
}

func TestNewRedisClient_WithEmptyPassword(t *testing.T) {
	t.Setenv("APP_ENV", "")

	cfg := &config.Config{
		RedisCfg: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}

	client := NewRedisClient(cfg)

	if client == nil {
		t.Error("Expected non-nil Redis client")
	}

	opts := client.Options()

	if opts.Password != "" {
		t.Errorf("Expected empty password, got %s", opts.Password)
	}
}

func TestNewRedisClient_InvalidEnvironmentIntegers(t *testing.T) {
	// Set environment with invalid integer values
	t.Setenv("APP_ENV", "development")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_DB", "invalid")
	t.Setenv("REDIS_MIN_IDLE_CONNECTIONS", "not-a-number")
	t.Setenv("REDIS_POOL_SIZE", "abc")
	t.Setenv("REDIS_POOL_TIMEOUT", "xyz")

	cfg := &config.Config{}

	client := NewRedisClient(cfg)

	if client == nil {
		t.Error("Expected non-nil Redis client")
	}

	// Client should still be created even with invalid values
	// utils.ConvertStringToInteger returns 0 for invalid strings
	opts := client.Options()

	// Just verify client was created successfully
	if opts.Addr != "localhost:6379" {
		t.Errorf("Expected address 'localhost:6379', got %s", opts.Addr)
	}
}
