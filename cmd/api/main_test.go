package main

import (
	"net"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"go.uber.org/zap/zapcore"
)

// TestMain_StartError covers the server-start failure branch (main.go:58-60). The
// real logger.Fatal would call os.Exit, so we swap the `fatal` seam for a stub
// that records the call instead. An empty SNMP config makes server.Start return
// an error.
func TestMain_StartError(t *testing.T) {
	orig := fatal
	called := false
	fatal = func(string, ...zapcore.Field) { called = true }
	defer func() { fatal = orig }()

	t.Setenv("APP_ENV", "test")
	t.Setenv("OLTS", "")
	t.Setenv("OLTS_FILE", "")
	t.Setenv("REGISTRY_URL", "")
	t.Setenv("SNMP_HOST", "") // no host -> no OLT can init -> Start returns error
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("TRAP_ENABLED", "false")

	main()

	if !called {
		t.Fatal("expected fatal() to be called on server start error")
	}
}

// TestMain_RunsAndShutsDown boots the real entrypoint with a working Redis
// (miniredis) and a stub SNMP UDP socket on an ephemeral port, then stops it with
// SIGTERM — exercising main() + the full server lifecycle (graceful shutdown) end
// to end.
func TestMain_RunsAndShutsDown(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp listener: %v", err)
	}
	defer func() { _ = udp.Close() }()
	snmpAddr := udp.LocalAddr().(*net.UDPAddr)

	t.Setenv("APP_ENV", "")      // empty -> defaults to "production" (covers the default branch)
	t.Setenv("SERVER_PORT", "0") // ephemeral — no conflict with anything running
	t.Setenv("SNMP_HOST", "127.0.0.1")
	t.Setenv("SNMP_PORT", strconv.Itoa(snmpAddr.Port))
	t.Setenv("SNMP_COMMUNITY", "public")
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("CACHE_PREWARM", "false")
	t.Setenv("OLTS", "")
	t.Setenv("OLTS_FILE", "")
	t.Setenv("REGISTRY_URL", "")
	t.Setenv("TRAP_ENABLED", "false")

	done := make(chan struct{})
	go func() { main(); close(done) }()

	time.Sleep(300 * time.Millisecond) // let the server bind + register its signal handler
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("main did not return after SIGTERM")
	}
}
