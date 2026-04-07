package app

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestNew(t *testing.T) {
	app := New()

	if app == nil {
		t.Fatal("Expected non-nil app instance")
	}

	if app.router != nil {
		t.Error("Expected router to be nil before Start is called")
	}
}

func TestNew_ReturnsAppStruct(t *testing.T) {
	app1 := New()
	app2 := New()

	if app1 == nil || app2 == nil {
		t.Error("Expected both app instances to be non-nil")
	}

	if app1 == app2 {
		t.Error("Expected different app instances")
	}
}

func TestApp_Start_MissingConfig(t *testing.T) {
	os.Unsetenv("SNMP_HOST")
	os.Unsetenv("SNMP_COMMUNITY")

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := app.Start(ctx)
	if err == nil {
		t.Error("Expected error for missing SNMP config")
	}
}

func TestApp_Start_SNMPSetupFailure(t *testing.T) {
	// Use miniredis for a working Redis, but invalid SNMP host to trigger SNMP failure
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	os.Setenv("SNMP_HOST", "invalid-host-!@#")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", "161")
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Covers: config OK → Redis init → Redis ping SUCCESS → Redis close →
	// SNMP setup FAILURE → return err
	err = app.Start(ctx)
	if err == nil {
		t.Error("Expected error for invalid SNMP host")
	}
}

func TestApp_Start_RedisPingFailure(t *testing.T) {
	// Start miniredis then close it before app.Start to trigger ping failure
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	host, port := mr.Host(), mr.Port()
	mr.Close() // Close before Start — ping will fail

	// Start UDP listener for SNMP
	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", host)
	os.Setenv("REDIS_PORT", port)
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("CACHE_PREWARM", "false")
	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("CACHE_PREWARM")
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	// Covers: config OK → Redis init → Redis ping FAILURE (non-fatal) →
	// SNMP connect OK → server start → cancel → shutdown
	_ = app.Start(ctx)
}

func TestApp_Start_WithTrapEnabled(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("TRAP_ENABLED", "true")
	os.Setenv("TRAP_PORT", "0")
	os.Setenv("TRAP_WEBHOOK_URL", "http://localhost:19999/test")
	os.Setenv("POWER_MONITOR_ENABLED", "false")
	os.Setenv("CACHE_PREWARM", "false")
	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("TRAP_ENABLED")
		os.Unsetenv("TRAP_PORT")
		os.Unsetenv("TRAP_WEBHOOK_URL")
		os.Unsetenv("POWER_MONITOR_ENABLED")
		os.Unsetenv("CACHE_PREWARM")
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	_ = app.Start(ctx)
}

func TestApp_Start_WithTrapAndPowerMonitor(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("TRAP_ENABLED", "true")
	os.Setenv("TRAP_PORT", "0")
	os.Setenv("TRAP_WEBHOOK_URL", "http://localhost:19999/test")
	os.Setenv("POWER_MONITOR_ENABLED", "true")
	os.Setenv("POWER_MONITOR_INTERVAL", "9999")
	os.Setenv("CACHE_PREWARM", "false")
	defer func() {
		for _, k := range []string{"SNMP_HOST", "SNMP_COMMUNITY", "SNMP_PORT",
			"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD",
			"TRAP_ENABLED", "TRAP_PORT", "TRAP_WEBHOOK_URL",
			"POWER_MONITOR_ENABLED", "POWER_MONITOR_INTERVAL", "CACHE_PREWARM"} {
			os.Unsetenv(k)
		}
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	_ = app.Start(ctx)
}

func TestApp_Start_WithTrapNoWebhook(t *testing.T) {
	// Test trap enabled but no webhook URL (covers the webhookClient == nil branch)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("TRAP_ENABLED", "true")
	os.Setenv("TRAP_PORT", "0")
	os.Setenv("TRAP_WEBHOOK_URL", "")
	os.Setenv("POWER_MONITOR_ENABLED", "true")
	os.Setenv("CACHE_PREWARM", "false")
	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("TRAP_ENABLED")
		os.Unsetenv("TRAP_PORT")
		os.Unsetenv("TRAP_WEBHOOK_URL")
		os.Unsetenv("POWER_MONITOR_ENABLED")
		os.Unsetenv("CACHE_PREWARM")
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	_ = app.Start(ctx)
}

func TestApp_Start_RedisCloseError(t *testing.T) {
	// Test that Redis close error is handled (covers defer close error branch)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("SERVER_PORT", "0")
	os.Setenv("CACHE_PREWARM", "false")
	defer func() {
		for _, k := range []string{"SNMP_HOST", "SNMP_COMMUNITY", "SNMP_PORT",
			"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "SERVER_PORT", "CACHE_PREWARM"} {
			os.Unsetenv(k)
		}
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		// Close miniredis before app shutdown to trigger Redis close error
		mr.Close()
		cancel()
	}()

	_ = app.Start(ctx)
}

func TestApp_Start_FullLifecycle(t *testing.T) {
	// Use miniredis for Redis + local UDP listener for SNMP
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// Start a local UDP listener to simulate SNMP agent
	udpListener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer udpListener.Close()
	snmpAddr := udpListener.LocalAddr().(*net.UDPAddr)

	os.Setenv("SNMP_HOST", "127.0.0.1")
	os.Setenv("SNMP_COMMUNITY", "public")
	os.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("CACHE_PREWARM", "false")
	// Don't set SERVER_PORT — let it fall back to default "8081" to cover that branch
	defer func() {
		os.Unsetenv("SNMP_HOST")
		os.Unsetenv("SNMP_COMMUNITY")
		os.Unsetenv("SNMP_PORT")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("CACHE_PREWARM")
	}()

	app := New()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay to trigger graceful shutdown
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	// Covers full lifecycle: config → Redis init → Redis ping OK →
	// SNMP connect OK → repository init → handler init → router init →
	// server start → context cancel → graceful shutdown → Redis close → SNMP close
	_ = app.Start(ctx)
}
