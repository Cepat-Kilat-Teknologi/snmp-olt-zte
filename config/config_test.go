package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "Environment variable exists",
			key:          "TEST_ENV",
			envValue:     "test_value",
			defaultValue: "default",
			expected:     "test_value",
		},
		{
			name:         "Environment variable empty - use default",
			key:          "TEST_ENV",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "Environment variable not set - use default",
			key:          "TEST_ENV_NOT_SET",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.key)
			}
			defer func() { _ = os.Unsetenv(tt.key) }()

			result := getEnv(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "Valid integer",
			key:          "TEST_INT",
			envValue:     "42",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "Empty value - use default",
			key:          "TEST_INT",
			envValue:     "",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "Invalid integer - use default",
			key:          "TEST_INT",
			envValue:     "invalid",
			defaultValue: 5,
			expected:     5,
		},
		{
			name:         "Negative integer",
			key:          "TEST_INT",
			envValue:     "-10",
			defaultValue: 0,
			expected:     -10,
		},
		{
			name:         "Zero",
			key:          "TEST_INT",
			envValue:     "0",
			defaultValue: 10,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.key)
			}
			defer func() { _ = os.Unsetenv(tt.key) }()

			result := getEnvAsInt(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsUint16(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue uint16
		expected     uint16
	}{
		{
			name:         "Valid uint16",
			key:          "TEST_UINT16",
			envValue:     "161",
			defaultValue: 100,
			expected:     161,
		},
		{
			name:         "Empty value - use default",
			key:          "TEST_UINT16",
			envValue:     "",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "Invalid uint16 - use default",
			key:          "TEST_UINT16",
			envValue:     "invalid",
			defaultValue: 50,
			expected:     50,
		},
		{
			name:         "Max uint16 value",
			key:          "TEST_UINT16",
			envValue:     "65535",
			defaultValue: 100,
			expected:     65535,
		},
		{
			name:         "Zero",
			key:          "TEST_UINT16",
			envValue:     "0",
			defaultValue: 100,
			expected:     0,
		},
		{
			name:         "Negative value - use default",
			key:          "TEST_UINT16",
			envValue:     "-1",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "Value exceeds uint16 - use default",
			key:          "TEST_UINT16",
			envValue:     "70000",
			defaultValue: 100,
			expected:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.key)
			}
			defer func() { _ = os.Unsetenv(tt.key) }()

			result := getEnvAsUint16(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsFloat64(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")
	if v := getEnvAsFloat64("TEST_FLOAT", 0); v != 3.14 {
		t.Errorf("Expected 3.14, got %f", v)
	}
	if v := getEnvAsFloat64("NONEXIST_FLOAT", 1.5); v != 1.5 {
		t.Errorf("Expected default 1.5, got %f", v)
	}
	os.Setenv("TEST_FLOAT_INVALID", "abc")
	defer os.Unsetenv("TEST_FLOAT_INVALID")
	if v := getEnvAsFloat64("TEST_FLOAT_INVALID", 2.0); v != 2.0 {
		t.Errorf("Expected default 2.0 for invalid float, got %f", v)
	}
}

func TestLoadConfig(t *testing.T) {
	// Set required environment variables
	os.Setenv("SNMP_HOST", "192.168.1.1")
	os.Setenv("SNMP_PORT", "161")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("REDIS_HOST", "localhost")
	os.Setenv("REDIS_PORT", "6379")

	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
	}()

	cfg, err := LoadConfig()

	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	// Check SNMP config
	if cfg.SnmpCfg.IP != "192.168.1.1" {
		t.Errorf("Expected SNMP IP '192.168.1.1', got '%s'", cfg.SnmpCfg.IP)
	}

	if cfg.SnmpCfg.Port != 161 {
		t.Errorf("Expected SNMP Port 161, got %d", cfg.SnmpCfg.Port)
	}

	if cfg.SnmpCfg.Community != "public" {
		t.Errorf("Expected SNMP Community 'public', got '%s'", cfg.SnmpCfg.Community)
	}

	// Check Redis config
	if cfg.RedisCfg.Host != "localhost" {
		t.Errorf("Expected Redis Host 'localhost', got '%s'", cfg.RedisCfg.Host)
	}

	if cfg.RedisCfg.Port != "6379" {
		t.Errorf("Expected Redis Port '6379', got '%s'", cfg.RedisCfg.Port)
	}

	// Check BoardPonMap is initialized
	if cfg.BoardPonMap == nil {
		t.Error("Expected BoardPonMap to be initialized")
	}

	// Check that all 32 configurations exist
	if len(cfg.BoardPonMap) != 32 {
		t.Errorf("Expected 32 board/pon configs, got %d", len(cfg.BoardPonMap))
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Set required SNMP env vars, clear everything else
	os.Setenv("SNMP_HOST", "10.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Unsetenv("SNMP_PORT")
	os.Unsetenv("REDIS_HOST")
	os.Unsetenv("REDIS_PORT")
	os.Unsetenv("REDIS_DB")
	os.Unsetenv("REDIS_MIN_IDLE_CONNECTIONS")
	os.Unsetenv("REDIS_POOL_SIZE")
	os.Unsetenv("REDIS_POOL_TIMEOUT")

	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
	}()

	cfg, err := LoadConfig()

	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check default values
	if cfg.SnmpCfg.Port != 161 {
		t.Errorf("Expected default SNMP Port 161, got %d", cfg.SnmpCfg.Port)
	}

	if cfg.RedisCfg.Host != "localhost" {
		t.Errorf("Expected default Redis Host 'localhost', got '%s'", cfg.RedisCfg.Host)
	}

	if cfg.RedisCfg.Port != "6379" {
		t.Errorf("Expected default Redis Port '6379', got '%s'", cfg.RedisCfg.Port)
	}

	if cfg.RedisCfg.MinIdleConnections != 10 {
		t.Errorf("Expected default MinIdleConnections 10, got %d", cfg.RedisCfg.MinIdleConnections)
	}

	if cfg.RedisCfg.PoolSize != 100 {
		t.Errorf("Expected default PoolSize 100, got %d", cfg.RedisCfg.PoolSize)
	}
}

func TestLoadConfig_MissingSNMPHost(t *testing.T) {
	os.Unsetenv("SNMP_HOST")
	os.Setenv("SNMP_COMMUNITY", "public")
	defer os.Unsetenv("SNMP_COMMUNITY")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error when SNMP_HOST is missing")
	}
	expected := "SNMP_HOST environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadConfig_MissingSNMPCommunity(t *testing.T) {
	os.Setenv("SNMP_HOST", "10.0.0.1")
	os.Unsetenv("SNMP_COMMUNITY")
	defer os.Unsetenv("SNMP_HOST")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("Expected error when SNMP_COMMUNITY is missing")
	}
	expected := "SNMP_COMMUNITY environment variable is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestGetBoardPonConfig(t *testing.T) {
	cfg := &Config{
		BoardPonMap: make(map[BoardPonKey]*BoardPonConfig),
	}

	// Add a test configuration
	testConfig := &BoardPonConfig{
		OnuIDNameOID:       "1.3.6.1.4.1.1",
		OnuTypeOID:         "1.3.6.1.4.1.2",
		OnuSerialNumberOID: "1.3.6.1.4.1.3",
	}

	cfg.BoardPonMap[BoardPonKey{BoardID: 1, PonID: 1}] = testConfig

	tests := []struct {
		name      string
		boardID   int
		ponID     int
		shouldErr bool
	}{
		{
			name:      "Valid board/pon combination",
			boardID:   1,
			ponID:     1,
			shouldErr: false,
		},
		{
			name:      "Invalid board/pon combination",
			boardID:   99,
			ponID:     99,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cfg.GetBoardPonConfig(tt.boardID, tt.ponID)

			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if result != nil {
					t.Error("Expected nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result")
				}
				if result != testConfig {
					t.Error("Expected to get the correct config")
				}
			}
		})
	}
}

func TestValidateConfig_Success(t *testing.T) {
	cfg := &Config{
		BoardPonMap: make(map[BoardPonKey]*BoardPonConfig),
	}

	// Add all 32 required configurations
	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			cfg.BoardPonMap[BoardPonKey{BoardID: boardID, PonID: ponID}] = &BoardPonConfig{
				OnuIDNameOID: "test",
			}
		}
	}

	err := cfg.ValidateConfig()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestValidateConfig_MissingConfig(t *testing.T) {
	cfg := &Config{
		BoardPonMap: make(map[BoardPonKey]*BoardPonConfig),
	}

	// Add only some configurations (missing Board1Pon1)
	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			if boardID == 1 && ponID == 1 {
				continue // Skip Board1Pon1
			}
			cfg.BoardPonMap[BoardPonKey{BoardID: boardID, PonID: ponID}] = &BoardPonConfig{
				OnuIDNameOID: "test",
			}
		}
	}

	err := cfg.ValidateConfig()

	if err == nil {
		t.Error("Expected error for missing configuration, got nil")
	}

	expectedError := "missing configuration for Board1Pon1"
	if err != nil && err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestBoardPonKey(t *testing.T) {
	key1 := BoardPonKey{BoardID: 1, PonID: 8}
	key2 := BoardPonKey{BoardID: 1, PonID: 8}
	key3 := BoardPonKey{BoardID: 2, PonID: 8}

	// Test that identical keys are equal (for map lookup)
	testMap := make(map[BoardPonKey]string)
	testMap[key1] = "test"

	if testMap[key2] != "test" {
		t.Error("Expected identical keys to access same map value")
	}

	if testMap[key3] == "test" {
		t.Error("Expected different keys to not access same map value")
	}
}

func TestConfig_StructFields(t *testing.T) {
	cfg := Config{
		SnmpCfg: SnmpConfig{
			IP:        "192.168.1.1",
			Port:      161,
			Community: "public",
		},
		RedisCfg: RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
		OltCfg: OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[BoardPonKey]*BoardPonConfig),
	}

	// Verify all struct fields are accessible
	if cfg.SnmpCfg.IP != "192.168.1.1" {
		t.Error("Failed to access SnmpCfg.IP")
	}

	if cfg.RedisCfg.Host != "localhost" {
		t.Error("Failed to access RedisCfg.Host")
	}

	if cfg.OltCfg.BaseOID1 != "1.3.6.1.4.1" {
		t.Error("Failed to access OltCfg.BaseOID1")
	}

	if cfg.BoardPonMap == nil {
		t.Error("BoardPonMap should not be nil")
	}
}
