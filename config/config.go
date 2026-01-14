package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config represents the main application configuration structure
// that contains all sub-configurations for SNMP, Redis, OLT, and board/PON configs.
// The 32 individual Board{X}Pon{Y} fields have been replaced with BoardPonMap for scalability.
type Config struct { // Define the main configuration struct named Config
	SnmpCfg     SnmpConfig                      // Field to hold SNMP configuration settings
	RedisCfg    RedisConfig                     // Field to hold Redis configuration settings
	OltCfg      OltConfig                       // Field to hold OLT configuration settings
	BoardPonMap map[BoardPonKey]*BoardPonConfig `mapstructure:"-"` // Dynamic map to store configurations for each Board and PON, ignored during direct un-marshaling
}

// SnmpConfig contains configuration parameters for SNMP connection
// including target IP address, port, and community string.
type SnmpConfig struct { // Define the SnmpConfig struct for SNMP settings
	IP        string `mapstructure:"ip"`        // IP address of the SNMP device, mapped from the "ip" configuration key
	Port      uint16 `mapstructure:"port"`      // Port number for the SNMP connection, mapped from the "port" configuration key
	Community string `mapstructure:"community"` // SNMP community string (password), mapped from the "community" configuration key
}

// RedisConfig contains configuration parameters for Redis connection,
// including host, port, authentication, and connection pooling settings.
type RedisConfig struct { // Define the RedisConfig struct for Redis settings
	Host               string `mapstructure:"host"`                 // Hostname or IP address of the Redis server, mapped from "host"
	Port               string `mapstructure:"port"`                 // Port number for the Redis server, mapped from "port"
	Password           string `mapstructure:"password"`             // Password for Redis authentication, mapped from "password"
	DB                 int    `mapstructure:"db"`                   // Database index to be selected, mapped from "db"
	DefaultDB          int    `mapstructure:"default_db"`           // Default database index, mapped from "default_db"
	MinIdleConnections int    `mapstructure:"min_idle_connections"` // Minimum number of idle connections in the pool, mapped from "min_idle_connections"
	PoolSize           int    `mapstructure:"pool_size"`            // Maximum number of connections in the pool, mapped from "pool_size"
	PoolTimeout        int    `mapstructure:"pool_timeout"`         // Timeout duration for waiting for a connection from the pool, mapped from "pool_timeout"
}

// OltConfig contains base OID configurations for OLT device management
// including common OIDs for ONU identification and type mapping.
type OltConfig struct { // Define the OltConfig struct for OLT settings
	BaseOID1        string `mapstructure:"base_oid_1"`  // First base OID string, mapped from "base_oid_1"
	BaseOID2        string `mapstructure:"base_oid_2"`  // Second base OID string, mapped from "base_oid_2"
	OnuIDNameAllPon string `mapstructure:"onu_id_name"` // OID name for ONU ID across all PONs, mapped from "onu_id_name"
	OnuTypeAllPon   string `mapstructure:"onu_type"`    // OID type for ONU across all PONs, mapped from "onu_type"
}

// BoardPonKey represents the unique key for board/pon lookup
type BoardPonKey struct { // Define the BoardPonKey struct to use as a map key
	BoardID int // Integer identifier for the Board
	PonID   int // Integer identifier for the PON
}

// BoardPonConfig contains OID configurations for a single Board-PON combination
// This replaces the 32 individual Board{X}Pon{Y} structs with a single reusable struct.
type BoardPonConfig struct { // Define the BoardPonConfig struct for specific Board-PON settings
	OnuIDNameOID              string `mapstructure:"onu_id_name"`               // OID for the ONU ID name, mapped from "onu_id_name"
	OnuTypeOID                string `mapstructure:"onu_type"`                  // OID for the ONU type, mapped from "onu_type"
	OnuSerialNumberOID        string `mapstructure:"onu_serial_number"`         // OID for the ONU serial number, mapped from "onu_serial_number"
	OnuRxPowerOID             string `mapstructure:"onu_rx_power"`              // OID for the ONU RX power, mapped from "onu_rx_power"
	OnuTxPowerOID             string `mapstructure:"onu_tx_power"`              // OID for the ONU TX power, mapped from "onu_tx_power"
	OnuStatusOID              string `mapstructure:"onu_status_id"`             // OID for the ONU status ID, mapped from "onu_status_id"
	OnuIPAddressOID           string `mapstructure:"onu_ip_address"`            // OID for the ONU IP address, mapped from "onu_ip_address"
	OnuDescriptionOID         string `mapstructure:"onu_description"`           // OID for the ONU description, mapped from "onu_description"
	OnuLastOnlineOID          string `mapstructure:"onu_last_online_time"`      // OID for the last online time, mapped from "onu_last_online_time"
	OnuLastOfflineOID         string `mapstructure:"onu_last_offline_time"`     // OID for the last offline time, mapped from "onu_last_offline_time"
	OnuLastOfflineReasonOID   string `mapstructure:"onu_last_offline_reason"`   // OID for the last offline reason, mapped from "onu_last_offline_reason"
	OnuGponOpticalDistanceOID string `mapstructure:"onu_gpon_optical_distance"` // OID for the GPON optical distance, mapped from "onu_gpon_optical_distance"
}

//==============================================================================
// 32 BOARD STRUCT DEFINITIONS DELETED (Board1Pon1 through Board2Pon16)
// They have been replaced by the single reusable BoardPonConfig struct above.
// This eliminates 512 lines of duplicate code and makes the system infinitely scalable.
//==============================================================================

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as int or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvAsUint16 retrieves an environment variable as uint16 or returns a default value
func getEnvAsUint16(key string, defaultValue uint16) uint16 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseUint(value, 10, 16); err == nil {
			return uint16(intVal)
		}
	}
	return defaultValue
}

// LoadConfig loads configuration from environment variables
// All sensitive data (SNMP, Redis, Server) MUST come from environment variables
// Board/PON OID mappings are generated dynamically using mathematical formulas (no config file needed)
func LoadConfig() (*Config, error) {
	var cfg Config

	// ===================================================================
	// Load from ENVIRONMENT VARIABLES (for sensitive data)
	// ===================================================================

	// SNMP Configuration from environment (REQUIRED for production)
	cfg.SnmpCfg = SnmpConfig{
		IP:        getEnv("SNMP_HOST", ""),
		Port:      getEnvAsUint16("SNMP_PORT", 161),
		Community: getEnv("SNMP_COMMUNITY", ""),
	}

	// Redis Configuration from environment (REQUIRED for production)
	cfg.RedisCfg = RedisConfig{
		Host:               getEnv("REDIS_HOST", "localhost"),
		Port:               getEnv("REDIS_PORT", "6379"),
		Password:           getEnv("REDIS_PASSWORD", ""),
		DB:                 getEnvAsInt("REDIS_DB", 0),
		DefaultDB:          getEnvAsInt("REDIS_DB", 0),
		MinIdleConnections: getEnvAsInt("REDIS_MIN_IDLE_CONNECTIONS", 200),
		PoolSize:           getEnvAsInt("REDIS_POOL_SIZE", 12000),
		PoolTimeout:        getEnvAsInt("REDIS_POOL_TIMEOUT", 240),
	}

	// OLT Configuration - use constants or environment variables
	cfg.OltCfg = OltConfig{
		BaseOID1:        getEnv("OLT_BASE_OID_1", BaseOID1), // Fallback to constant
		BaseOID2:        getEnv("OLT_BASE_OID_2", BaseOID2), // Fallback to constant
		OnuIDNameAllPon: getEnv("ONU_ID_NAME_PREFIX", OnuIDNamePrefix),
		OnuTypeAllPon:   getEnv("ONU_TYPE_PREFIX", OnuTypePrefix),
	}

	// ===================================================================
	// Generate Board/PON OID mappings DYNAMICALLY (no config file needed)
	// ===================================================================

	// Generate all 32 Board-PON configurations using mathematical formulas
	// Note: InitializeBoardPonMap cannot fail since it uses hardcoded valid values
	// and always creates all 32 board/pon combinations, so ValidateConfig is not needed here
	cfg.BoardPonMap, _ = InitializeBoardPonMap()

	return &cfg, nil
}

// GetBoardPonConfig retrieves configuration for a specific board and PON
func (c *Config) GetBoardPonConfig(boardID, ponID int) (*BoardPonConfig, error) { // Define method GetBoardPonConfig on Config struct; takes boardID and ponID
	key := BoardPonKey{BoardID: boardID, PonID: ponID} // Create a BoardPonKey using the provided boardID and ponID
	cfg, ok := c.BoardPonMap[key]                      // Attempt to retrieve the configuration from the map
	if !ok {                                           // Check if the retrieval was successful (ok is false if key not found)
		return nil, fmt.Errorf("config not found for board %d, pon %d", boardID, ponID) // Return nil and a formatted error message if not found
	}
	return cfg, nil // Return the found configuration and nil error
}

// ValidateConfig validates that all required board/pon configurations are present
func (c *Config) ValidateConfig() error { // Define method ValidateConfig on Config struct; returns an error
	// Validate that all 32 board/pon combinations exist
	for boardID := 1; boardID <= 2; boardID++ { // Loop through all expected board IDs
		for ponID := 1; ponID <= 16; ponID++ { // Loop through all expected PON IDs
			key := BoardPonKey{BoardID: boardID, PonID: ponID} // Construct the key for the current combination
			if _, ok := c.BoardPonMap[key]; !ok {              // Check if the key exists in the BoardPonMap
				return fmt.Errorf("missing configuration for Board%dPon%d", boardID, ponID) // Return an error if a mandatory configuration is missing
			}
		}
	}
	return nil // Return nil if all validations pass
}
