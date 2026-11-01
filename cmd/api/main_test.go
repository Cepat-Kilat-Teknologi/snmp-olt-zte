package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
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

	port := freeTCPPort(t)
	t.Setenv("APP_ENV", "") // empty -> defaults to "production" (covers the default branch)
	t.Setenv("SERVER_PORT", strconv.Itoa(port))
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

	// A bind failure would reach logger.Fatal and exit the test binary.
	orig := fatal
	fatal = func(msg string, _ ...zapcore.Field) { t.Errorf("fatal: %s", msg) }
	defer func() { fatal = orig }()

	// Keep a SIGTERM that arrives before main registers its handler from
	// terminating the test binary. Signals are delivered to every channel
	// registered for them, so main still receives the ones sent later.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	done := make(chan struct{})
	go func() { main(); close(done) }()

	waitHealthy(t, "http://127.0.0.1:"+strconv.Itoa(port)+"/healthz", 30*time.Second)

	// The HTTP server can answer before graceful.Shutdown calls
	// signal.Notify, so repeat SIGTERM until main returns.
	deadline := time.After(30 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
			t.Fatalf("signal: %v", err)
		}
		select {
		case <-done:
			return
		case <-deadline:
			t.Fatal("main did not return after SIGTERM")
		case <-tick.C:
		}
	}
}

// freeTCPPort returns a loopback port that was free a moment ago. main binds
// the port itself, so the test needs to know it in advance to poll /healthz.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// waitHealthy polls url until it answers 200 or the timeout passes.
func waitHealthy(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url) //nolint:noctx // test-only poll with a client timeout
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s not healthy within %s", url, timeout)
}
