package redis

import (
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and returns a new Redis client based on the provided configuration.
// It checks environment variables first (if in dev/prod mode) or falls back to the provided config struct.
func NewRedisClient(cfg *config.Config) *redis.Client {
	var (
		redisHost               string // Redis Host
		redisPort               string // Redis Port
		redisPassword           string // Redis Password
		redisDB                 int    // Redis Database Index
		redisMinIdleConnections int    // Minimum idle connections
		redisPoolSize           int    // Max pool size
		redisPoolTimeout        int    // Pool timeout duration
	)

	// Check application environment
	if os.Getenv("APP_ENV") == "development" || os.Getenv("APP_ENV") == "production" {
		// Load from environment variables if in specific environments
		redisHost = os.Getenv("REDIS_HOST")
		redisPort = os.Getenv("REDIS_PORT")
		redisPassword = os.Getenv("REDIS_PASSWORD")
		redisDB = utils.ConvertStringToInteger(os.Getenv("REDIS_DB"))
		redisMinIdleConnections = utils.ConvertStringToInteger(os.Getenv("REDIS_MIN_IDLE_CONNECTIONS"))
		redisPoolSize = utils.ConvertStringToInteger(os.Getenv("REDIS_POOL_SIZE"))
		redisPoolTimeout = utils.ConvertStringToInteger(os.Getenv("REDIS_POOL_TIMEOUT"))
	} else {
		// Load from configuration struct otherwise (e.g., local testing or other envs)
		redisHost = cfg.RedisCfg.Host
		redisPort = cfg.RedisCfg.Port
		redisPassword = cfg.RedisCfg.Password
		redisDB = cfg.RedisCfg.DB
		redisMinIdleConnections = cfg.RedisCfg.MinIdleConnections
		redisPoolSize = cfg.RedisCfg.PoolSize
		redisPoolTimeout = cfg.RedisCfg.PoolTimeout
	}

	// Create and return the Redis client with the resolved options
	return redis.NewClient(&redis.Options{
		Addr:         redisHost + ":" + redisPort,                   // Construct address
		Password:     redisPassword,                                 // Set password
		DB:           redisDB,                                       // Set database
		MinIdleConns: redisMinIdleConnections,                       // Set min idle conns
		PoolSize:     redisPoolSize,                                 // Set pool size
		PoolTimeout:  time.Duration(redisPoolTimeout) * time.Second, // Set pool timeout
	})
}
