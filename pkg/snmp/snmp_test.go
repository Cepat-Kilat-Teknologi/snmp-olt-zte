package snmp

import (
	"net"
	"strconv"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
)

func TestSetupSnmpConnection_FromEnvironment(t *testing.T) {
	// Set environment variables
	t.Setenv("APP_ENV", "production")
	t.Setenv("SNMP_HOST", "192.168.1.1")
	t.Setenv("SNMP_PORT", "161")
	t.Setenv("SNMP_COMMUNITY", "public")

	cfg := &config.Config{}

	// Note: This will try to actually connect to 192.168.1.1:161
	// In a real test environment, you might want to mock the connection
	conn, err := SetupSnmpConnection(cfg)

	// If device is not available, error is expected
	if err != nil {
		// Check that error is connection-related (expected in test)
		if conn != nil {
			t.Error("Expected nil connection on error")
		}
		// This is acceptable - device might not be available in test
		return
	}

	// If connection succeeded (unlikely in test), verify it's configured correctly
	if conn == nil {
		t.Error("Expected non-nil connection")
	}

	if conn != nil {
		defer func() { _ = conn.Conn.Close() }()

		if conn.Target != "192.168.1.1" {
			t.Errorf("Expected target 192.168.1.1, got %s", conn.Target)
		}

		if conn.Port != 161 {
			t.Errorf("Expected port 161, got %d", conn.Port)
		}

		if conn.Community != "public" {
			t.Errorf("Expected community 'public', got %s", conn.Community)
		}

		if conn.Timeout.Seconds() != 5 {
			t.Errorf("Expected timeout 5s, got %v", conn.Timeout)
		}

		if conn.Retries != 2 {
			t.Errorf("Expected retries 2, got %d", conn.Retries)
		}

		if conn.MaxOids != 60 {
			t.Errorf("Expected MaxOids 60, got %d", conn.MaxOids)
		}
	}
}

func TestSetupSnmpConnection_FromConfig(t *testing.T) {
	// Ensure no environment variables set
	t.Setenv("APP_ENV", "test")

	cfg := &config.Config{
		SnmpCfg: config.SnmpConfig{
			IP:        "10.0.0.1",
			Port:      161,
			Community: "private",
		},
	}

	conn, err := SetupSnmpConnection(cfg)

	// Connection will fail in test, but we can verify error handling
	if err != nil {
		// Expected - device not available
		if conn != nil {
			t.Error("Expected nil connection on error")
		}
		return
	}

	// If somehow succeeded, verify configuration
	if conn != nil {
		defer func() { _ = conn.Conn.Close() }()

		if conn.Target != "10.0.0.1" {
			t.Errorf("Expected target 10.0.0.1, got %s", conn.Target)
		}

		if conn.Port != 161 {
			t.Errorf("Expected port 161, got %d", conn.Port)
		}

		if conn.Community != "private" {
			t.Errorf("Expected community 'private', got %s", conn.Community)
		}
	}
}

func TestSetupSnmpConnection_InvalidConfig(t *testing.T) {
	// Set invalid environment
	t.Setenv("APP_ENV", "production")
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_PORT", "0")
	t.Setenv("SNMP_COMMUNITY", "")

	cfg := &config.Config{}

	conn, err := SetupSnmpConnection(cfg)

	if err == nil {
		t.Error("Expected error for invalid config")
	}

	if conn != nil {
		t.Error("Expected nil connection for invalid config")
	}
}

func TestSetupSnmpConnection_MissingHost(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_PORT", "161")
	t.Setenv("SNMP_COMMUNITY", "public")

	cfg := &config.Config{}

	conn, err := SetupSnmpConnection(cfg)

	if err == nil {
		t.Error("Expected error for missing host")
	}

	if conn != nil {
		t.Error("Expected nil connection for missing host")
	}
}

func TestSetupSnmpConnection_MissingPort(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SNMP_HOST", "192.168.1.1")
	t.Setenv("SNMP_PORT", "0")
	t.Setenv("SNMP_COMMUNITY", "public")

	cfg := &config.Config{}

	conn, err := SetupSnmpConnection(cfg)

	if err == nil {
		t.Error("Expected error for missing port")
	}

	if conn != nil {
		t.Error("Expected nil connection for missing port")
	}
}

func TestSetupSnmpConnection_MissingCommunity(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SNMP_HOST", "192.168.1.1")
	t.Setenv("SNMP_PORT", "161")
	t.Setenv("SNMP_COMMUNITY", "")

	cfg := &config.Config{}

	conn, err := SetupSnmpConnection(cfg)

	if err == nil {
		t.Error("Expected error for missing community")
	}

	if conn != nil {
		t.Error("Expected nil connection for missing community")
	}
}

func TestSetupSnmpConnection_Success(t *testing.T) {
	// Start a local UDP listener to accept SNMP connections
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.LocalAddr().(*net.UDPAddr)

	t.Setenv("APP_ENV", "test")

	cfg := &config.Config{
		SnmpCfg: config.SnmpConfig{
			IP:        "127.0.0.1",
			Port:      uint16(addr.Port),
			Community: "public",
		},
	}

	conn, err := SetupSnmpConnection(cfg)
	if err != nil {
		t.Fatalf("Expected successful connection, got error: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}
	defer func() { _ = conn.Conn.Close() }()

	if conn.Target != "127.0.0.1" {
		t.Errorf("Expected target 127.0.0.1, got %s", conn.Target)
	}
	if conn.Port != uint16(addr.Port) {
		t.Errorf("Expected port %d, got %d", addr.Port, conn.Port)
	}
	if conn.Community != "public" {
		t.Errorf("Expected community 'public', got %s", conn.Community)
	}
	if conn.Timeout.Seconds() != 5 {
		t.Errorf("Expected timeout 5s, got %v", conn.Timeout)
	}
	if conn.Retries != 2 {
		t.Errorf("Expected retries 2, got %d", conn.Retries)
	}
	if conn.MaxOids != 60 {
		t.Errorf("Expected MaxOids 60, got %d", conn.MaxOids)
	}
}

func TestSetupSnmpConnection_ConnectFailure(t *testing.T) {
	// Valid config but using a hostname that causes Connect() to fail
	t.Setenv("APP_ENV", "test")

	cfg := &config.Config{
		SnmpCfg: config.SnmpConfig{
			IP:        "invalid-hostname-!@#$%", // Will cause DNS/connect error
			Port:      161,
			Community: "public",
		},
	}

	conn, err := SetupSnmpConnection(cfg)

	if err == nil {
		if conn != nil {
			_ = conn.Conn.Close()
		}
		t.Error("Expected error for invalid hostname connect failure")
	}
	if conn != nil {
		t.Error("Expected nil connection on connect failure")
	}
}

func TestSetupSnmpConnection_Development(t *testing.T) {
	// Start local UDP listener so connection succeeds
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.LocalAddr().(*net.UDPAddr)
	port := strconv.Itoa(addr.Port)

	t.Setenv("APP_ENV", "development")
	t.Setenv("SNMP_HOST", "127.0.0.1")
	t.Setenv("SNMP_PORT", port)
	t.Setenv("SNMP_COMMUNITY", "test")

	cfg := &config.Config{}

	conn, err := SetupSnmpConnection(cfg)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}
	defer func() { _ = conn.Conn.Close() }()

	if conn.Target != "127.0.0.1" {
		t.Errorf("Expected target 127.0.0.1, got %s", conn.Target)
	}
}
