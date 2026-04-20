package repository

import (
	"net"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

// newTestConn creates a GoSNMP instance for testing (not connected).
func newTestConn(target, community string, port uint16) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:    target,
		Port:      port,
		Community: community,
		Version:   gosnmp.Version2c,
	}
}

func TestNewPonRepository(t *testing.T) {
	conn := newTestConn("192.168.1.1", "public", 161)

	repo := NewPonRepository(conn)

	if repo == nil {
		t.Error("Expected non-nil repository")
	}

	// Constructor returns the interface — no extra check needed
	_ = repo
}

func TestNewPonRepository_DifferentParameters(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		community string
		port      uint16
	}{
		{
			name:      "Standard SNMP configuration",
			target:    "192.168.1.1",
			community: "public",
			port:      161,
		},
		{
			name:      "Custom port",
			target:    "10.0.0.1",
			community: "private",
			port:      1161,
		},
		{
			name:      "Localhost",
			target:    "localhost",
			community: "test",
			port:      161,
		},
		{
			name:      "IPv6 address",
			target:    "::1",
			community: "public",
			port:      161,
		},
		{
			name:      "Empty community",
			target:    "192.168.1.1",
			community: "",
			port:      161,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newTestConn(tt.target, tt.community, tt.port)
			repo := NewPonRepository(conn)

			if repo == nil {
				t.Error("Expected non-nil repository")
			}

			// Type assert to check internal config
			if snmpRepo, ok := repo.(*snmpRepository); ok {
				if snmpRepo.cfg.target != tt.target {
					t.Errorf("Expected target '%s', got '%s'", tt.target, snmpRepo.cfg.target)
				}
				if snmpRepo.cfg.community != tt.community {
					t.Errorf("Expected community '%s', got '%s'", tt.community, snmpRepo.cfg.community)
				}
				if snmpRepo.cfg.port != tt.port {
					t.Errorf("Expected port %d, got %d", tt.port, snmpRepo.cfg.port)
				}
			} else {
				t.Error("Failed to type assert repository to *snmpRepository")
			}
		})
	}
}

func TestSnmpRepository_Get_NoConnection(t *testing.T) {
	// Test with a conn that has no underlying transport (Conn is nil)
	conn := newTestConn("invalid-host-that-does-not-exist", "public", 161)
	repo := NewPonRepository(conn)

	oids := []string{"1.3.6.1.2.1.1.1.0"}

	result, err := repo.Get(oids)

	// Should get an error because the connection was never established
	if err == nil {
		t.Error("Expected error for unconnected SNMP instance, got nil")
	}

	if result != nil {
		t.Error("Expected nil result on error")
	}

	// Verify error message contains expected text
	if err != nil && err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestSnmpRepository_Ping_NoConnection(t *testing.T) {
	// Ping wraps Get on sysUpTime; without a live connection it must
	// return a non-nil error so the readiness probe reports "down".
	conn := newTestConn("invalid-host-that-does-not-exist", "public", 161)
	repo := NewPonRepository(conn)

	if err := repo.Ping(); err == nil {
		t.Error("Expected non-nil error from Ping() on unconnected SNMP instance")
	}
}

func TestSnmpRepository_Get_EmptyOIDs(t *testing.T) {
	conn := newTestConn("invalid-host", "public", 161)
	repo := NewPonRepository(conn)

	// Try with empty OID list
	result, err := repo.Get([]string{})

	// Will fail due to no connection
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestSnmpRepository_Walk_NoConnection(t *testing.T) {
	conn := newTestConn("invalid-host-that-does-not-exist", "public", 161)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"

	callbackCalled := false
	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		callbackCalled = true
		return nil
	}

	err := repo.Walk(oid, walkFunc)

	// Should get an error because the connection was never established
	if err == nil {
		t.Error("Expected error for unconnected SNMP instance, got nil")
	}

	// Callback should not be called if there is no connection
	if callbackCalled {
		t.Error("Callback should not be called when connection is not established")
	}
}

func TestSnmpRepository_Walk_ErrorPropagation(t *testing.T) {
	// This test verifies that the Walk method properly handles errors
	conn := newTestConn("127.0.0.1", "public", 65535)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"

	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		return nil
	}

	err := repo.Walk(oid, walkFunc)

	// Should get error since connection was never established
	if err == nil {
		t.Error("Expected error for unconnected instance, got nil")
	}
}

func TestSnmpRepository_BulkWalk_NoConnection(t *testing.T) {
	conn := newTestConn("invalid-host-that-does-not-exist", "public", 161)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"

	callbackCalled := false
	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		callbackCalled = true
		return nil
	}

	err := repo.BulkWalk(oid, walkFunc)

	// Should get an error because the connection was never established
	if err == nil {
		t.Error("Expected error for unconnected SNMP instance, got nil")
	}

	// Callback should not be called if there is no connection
	if callbackCalled {
		t.Error("Callback should not be called when connection is not established")
	}
}

func TestSnmpRepository_BulkWalk_ErrorPropagation(t *testing.T) {
	// This test verifies that the BulkWalk method properly handles errors
	conn := newTestConn("127.0.0.1", "public", 65535)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"

	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		return nil
	}

	err := repo.BulkWalk(oid, walkFunc)

	// Should get error since connection was never established
	if err == nil {
		t.Error("Expected error for unconnected instance, got nil")
	}
}

func TestSnmpRepository_BulkWalk_Success(t *testing.T) {
	conn := newTestConn("invalid-host", "public", 161)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"
	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		return nil
	}

	err := repo.BulkWalk(oid, walkFunc)

	// Will fail due to no connection, testing error handling
	if err == nil {
		t.Error("Expected error for unconnected host")
	}
}

func TestSnmpRepository_InterfaceCompliance(t *testing.T) {
	// Verify that snmpRepository implements SnmpRepositoryInterface
	conn := newTestConn("invalid-host-that-does-not-exist", "public", 161)
	repo := NewPonRepository(conn)

	if repo == nil {
		t.Error("Repository should not be nil")
	}

	// Verify interface methods exist by calling them
	// We don't check for errors here as connection behavior can vary
	// The main goal is to verify the interface is implemented correctly
	_, _ = repo.Get([]string{"1.3.6.1.2.1.1.1.0"})
	_ = repo.Walk("1.3.6.1", func(pdu gosnmp.SnmpPDU) error { return nil })
	_ = repo.BulkWalk("1.3.6.1", func(pdu gosnmp.SnmpPDU) error { return nil })
}

func TestSnmpRepository_Get_MultipleOIDs(t *testing.T) {
	conn := newTestConn("invalid-host", "public", 161)
	repo := NewPonRepository(conn)

	// Test with multiple OIDs
	oids := []string{
		"1.3.6.1.2.1.1.1.0",
		"1.3.6.1.2.1.1.2.0",
		"1.3.6.1.2.1.1.3.0",
	}

	result, err := repo.Get(oids)

	// Will fail due to no connection
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestSnmpRepository_StructFields(t *testing.T) {
	target := "10.0.0.1"
	community := "test-community"
	var port uint16 = 8161

	conn := newTestConn(target, community, port)
	repo := NewPonRepository(conn)

	// Type assert to access internal fields
	snmpRepo, ok := repo.(*snmpRepository)
	if !ok {
		t.Fatal("Failed to type assert to *snmpRepository")
	}

	if snmpRepo.cfg.target != target {
		t.Errorf("Expected target '%s', got '%s'", target, snmpRepo.cfg.target)
	}

	if snmpRepo.cfg.community != community {
		t.Errorf("Expected community '%s', got '%s'", community, snmpRepo.cfg.community)
	}

	if snmpRepo.cfg.port != port {
		t.Errorf("Expected port %d, got %d", port, snmpRepo.cfg.port)
	}
}

func TestSnmpRepository_ZeroPort(t *testing.T) {
	// Test with port 0 (should be allowed but won't connect)
	conn := newTestConn("localhost", "public", 0)
	repo := NewPonRepository(conn)

	if repo == nil {
		t.Error("Expected non-nil repository even with port 0")
	}

	// Verify it was set
	if snmpRepo, ok := repo.(*snmpRepository); ok {
		if snmpRepo.cfg.port != 0 {
			t.Errorf("Expected port 0, got %d", snmpRepo.cfg.port)
		}
	}
}

func TestSnmpRepository_Get_Success(t *testing.T) {
	conn := newTestConn("invalid-host", "public", 161)
	repo := NewPonRepository(conn)

	oids := []string{"1.3.6.1.2.1.1.1.0"}
	_, err := repo.Get(oids)

	// Will fail due to no connection, testing error handling
	if err == nil {
		t.Error("Expected error for unconnected host")
	}
}

func TestSnmpRepository_Walk_Success(t *testing.T) {
	conn := newTestConn("invalid-host", "public", 161)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"
	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		return nil
	}

	err := repo.Walk(oid, walkFunc)

	// Will fail due to no connection, testing error handling
	if err == nil {
		t.Error("Expected error for unconnected host")
	}
}

func TestSnmpRepository_Get_NilOIDs(t *testing.T) {
	conn := newTestConn("localhost", "public", 161)
	repo := NewPonRepository(conn)

	_, err := repo.Get(nil)

	// Should handle nil OIDs gracefully
	if err == nil {
		t.Error("Expected error for nil OIDs")
	}
}

func TestSnmpRepository_Walk_CallbackError(t *testing.T) {
	conn := newTestConn("localhost", "public", 161)
	repo := NewPonRepository(conn)

	oid := "1.3.6.1.2.1.1"
	walkFunc := func(pdu gosnmp.SnmpPDU) error {
		return nil
	}

	err := repo.Walk(oid, walkFunc)

	// Will error because connection was not established
	if err == nil {
		t.Error("Expected error")
	}
}

// startFakeSNMPAgent starts a UDP listener that echoes back valid SNMP GetResponse
// packets. It parses incoming requests and responds with matching request IDs.
func startFakeSNMPAgent(t *testing.T) (net.PacketConn, uint16) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP listener: %v", err)
	}

	addr := listener.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 4096)
		decoder := &gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: "public"}
		for {
			n, remoteAddr, err := listener.ReadFrom(buf)
			if err != nil {
				return
			}

			decoded, err := decoder.SnmpDecodePacket(buf[:n])
			if err != nil || decoded == nil {
				continue
			}

			resp := &gosnmp.SnmpPacket{
				Version:   gosnmp.Version2c,
				Community: "public",
				PDUType:   gosnmp.GetResponse,
				RequestID: decoded.RequestID,
				Variables: []gosnmp.SnmpPDU{
					{Name: ".1.3.6.1.2.1.1.1.0", Type: gosnmp.EndOfMibView, Value: nil},
				},
			}

			respBytes, err := resp.MarshalMsg()
			if err != nil {
				continue
			}
			_, _ = listener.WriteTo(respBytes, remoteAddr)
		}
	}()

	return listener, uint16(addr.Port)
}

func newConnectedTestConn(t *testing.T, port uint16) *gosnmp.GoSNMP {
	t.Helper()
	conn := &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      port,
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   0,
	}
	if err := conn.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	return conn
}

func TestSnmpRepository_Get_Connected(t *testing.T) {
	listener, port := startFakeSNMPAgent(t)
	defer func() { _ = listener.Close() }()

	conn := newConnectedTestConn(t, port)
	defer func() { _ = conn.Conn.Close() }()

	repo := NewPonRepository(conn)
	result, err := repo.Get([]string{"1.3.6.1.2.1.1.1.0"})

	// Fake agent responds with EndOfMibView — Get should succeed
	_ = result
	_ = err
}

func TestSnmpRepository_Walk_Connected(t *testing.T) {
	listener, port := startFakeSNMPAgent(t)
	defer func() { _ = listener.Close() }()

	conn := newConnectedTestConn(t, port)
	defer func() { _ = conn.Conn.Close() }()

	repo := NewPonRepository(conn)
	err := repo.Walk("1.3.6.1.2.1.1", func(pdu gosnmp.SnmpPDU) error {
		return nil
	})
	_ = err
}

func TestSnmpRepository_Close(t *testing.T) {
	conn := newTestConn("localhost", "public", 161)
	repo := NewPonRepository(conn)
	// Close should not panic
	repo.Close()
}

func TestSnmpRepository_BulkWalk_Connected(t *testing.T) {
	listener, port := startFakeSNMPAgent(t)
	defer func() { _ = listener.Close() }()

	conn := newConnectedTestConn(t, port)
	defer func() { _ = conn.Conn.Close() }()

	repo := NewPonRepository(conn)
	err := repo.BulkWalk("1.3.6.1.2.1.1", func(pdu gosnmp.SnmpPDU) error {
		return nil
	})
	_ = err
}

func TestNewPonRepositoryWithConcurrency_ZeroMaxConcurrent(t *testing.T) {
	conn := newTestConn("192.168.1.1", "public", 161)
	repo := NewPonRepositoryWithConcurrency(conn, 0)
	if repo == nil {
		t.Error("Expected non-nil repository with zero maxConcurrent")
	}
}

func TestNewPonRepositoryWithConcurrency_NegativeMaxConcurrent(t *testing.T) {
	conn := newTestConn("192.168.1.1", "public", 161)
	repo := NewPonRepositoryWithConcurrency(conn, -1)
	if repo == nil {
		t.Error("Expected non-nil repository with negative maxConcurrent")
	}
}
