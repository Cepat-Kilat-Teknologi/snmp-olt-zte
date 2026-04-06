package repository

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// SnmpRepositoryInterface is an interface that represents the SNMP repository contract
type SnmpRepositoryInterface interface {
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)
	Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error
	BulkWalk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error
	Close()
}

// snmpConfig holds connection parameters for creating new SNMP connections
type snmpConfig struct {
	target    string
	port      uint16
	community string
	version   gosnmp.SnmpVersion
	timeout   time.Duration
	retries   int
	maxOids   int
}

// snmpRepository uses a connection pool for concurrent SNMP access
type snmpRepository struct {
	pool chan *gosnmp.GoSNMP
	cfg  snmpConfig
}

// DefaultPoolSize is the default number of SNMP connections in the pool
const DefaultPoolSize = 4

// NewPonRepository creates a repository with a connection pool.
// The seed connection is used to extract config, then poolSize connections are created.
func NewPonRepository(seed *gosnmp.GoSNMP) SnmpRepositoryInterface {
	cfg := snmpConfig{
		target:    seed.Target,
		port:      seed.Port,
		community: seed.Community,
		version:   seed.Version,
		timeout:   seed.Timeout,
		retries:   seed.Retries,
		maxOids:   seed.MaxOids,
	}

	pool := make(chan *gosnmp.GoSNMP, DefaultPoolSize)

	// Put the seed connection as the first pool member
	pool <- seed

	// Create additional connections to fill the pool
	for i := 1; i < DefaultPoolSize; i++ {
		conn, err := createConnection(cfg)
		if err != nil {
			// If we can't create more connections, proceed with what we have
			break
		}
		pool <- conn
	}

	return &snmpRepository{
		pool: pool,
		cfg:  cfg,
	}
}

// createConnection creates a new SNMP connection from config
func createConnection(cfg snmpConfig) (*gosnmp.GoSNMP, error) {
	conn := &gosnmp.GoSNMP{
		Target:    cfg.target,
		Port:      cfg.port,
		Community: cfg.community,
		Version:   cfg.version,
		Timeout:   cfg.timeout,
		Retries:   cfg.retries,
		MaxOids:   cfg.maxOids,
	}
	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP pool connect failed: %w", err)
	}
	return conn, nil
}

// acquire gets a connection from the pool
func (r *snmpRepository) acquire() *gosnmp.GoSNMP {
	return <-r.pool
}

// release returns a connection to the pool
func (r *snmpRepository) release(conn *gosnmp.GoSNMP) {
	r.pool <- conn
}

// Get retrieves SNMP data for the given OIDs
func (r *snmpRepository) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	conn := r.acquire()
	defer r.release(conn)

	result, err := conn.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("SNMP Get failed: %w", err)
	}
	return result, nil
}

// Walk performs SNMP Walk to get all OIDs under the given OID
func (r *snmpRepository) Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	conn := r.acquire()
	defer r.release(conn)

	err := conn.Walk(oid, walkFunc)
	if err != nil {
		return fmt.Errorf("SNMP Walk failed: %w", err)
	}
	return nil
}

// Close drains the pool and closes all SNMP connections
func (r *snmpRepository) Close() {
	close(r.pool)
	for conn := range r.pool {
		if conn.Conn != nil {
			conn.Conn.Close()
		}
	}
}

// BulkWalk performs SNMP BulkWalk to get all OIDs under the given OID using GetBulk requests
func (r *snmpRepository) BulkWalk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	conn := r.acquire()
	defer r.release(conn)

	err := conn.BulkWalk(oid, walkFunc)
	if err != nil {
		return fmt.Errorf("SNMP BulkWalk failed: %w", err)
	}
	return nil
}
