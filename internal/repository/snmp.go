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
	// Ping performs a lightweight reachability check against the OLT by
	// issuing a single SNMP Get for sysUpTime (1.3.6.1.2.1.1.3.0). It is
	// used by the readiness probe; callers should treat the result as a
	// best-effort signal and not rely on it for correctness.
	Ping() error
	Close()
}

// snmpConfig holds connection parameters for creating new SNMP connections
type snmpConfig struct {
	target         string
	port           uint16
	community      string
	version        gosnmp.SnmpVersion
	timeout        time.Duration
	retries        int
	maxOids        int
	maxRepetitions uint32
}

// snmpRepository uses a connection pool for concurrent SNMP access
type snmpRepository struct {
	pool    chan *gosnmp.GoSNMP
	cfg     snmpConfig
	sem     chan struct{} // limits concurrent SNMP operations to prevent OLT saturation
	useWalk bool          // use GetNext walk instead of GetBulk (robust over slow/public links)
}

// DefaultPoolSize is the default number of SNMP connections in the pool
const DefaultPoolSize = 4

// DefaultMaxConcurrent is the default limit for concurrent SNMP operations
const DefaultMaxConcurrent = 5

// DefaultMaxRepetitions is the GETBULK max-repetitions applied to pool
// connections when the seed connection carries no explicit value. It is
// intentionally never 0: GetBulk with max-repetitions=0 hangs on some ZTE
// OLTs (e.g. C300 V2.1.0), so the per-OLT walk=true (GetNext) workaround was
// needed to read them. Setting a sane default makes GetBulk usable again.
const DefaultMaxRepetitions uint32 = 20

// NewPonRepository creates a repository with a connection pool and default concurrency limit.
// The seed connection is used to extract config, then poolSize connections are created.
func NewPonRepository(seed *gosnmp.GoSNMP) SnmpRepositoryInterface {
	return NewPonRepositoryWithConcurrency(seed, DefaultMaxConcurrent, false)
}

// NewPonRepositoryWithConcurrency creates a repository with a connection pool and
// custom concurrency limit. useWalk forces GetNext instead of GetBulk for OLTs on
// lossy/high-latency links (set per-OLT in the registry).
func NewPonRepositoryWithConcurrency(seed *gosnmp.GoSNMP, maxConcurrent int, useWalk bool) SnmpRepositoryInterface {
	cfg := snmpConfig{
		target:         seed.Target,
		port:           seed.Port,
		community:      seed.Community,
		version:        seed.Version,
		timeout:        seed.Timeout,
		retries:        seed.Retries,
		maxOids:        seed.MaxOids,
		maxRepetitions: seed.MaxRepetitions,
	}
	// Never propagate a 0 max-repetitions to the pool: a GetBulk with
	// max-repetitions=0 hangs on some ZTE OLTs. If the seed was built without
	// it (e.g. a hand-rolled connection), fall back to a sane default so every
	// pooled connection can perform GetBulk reliably.
	if cfg.maxRepetitions == 0 {
		cfg.maxRepetitions = DefaultMaxRepetitions
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

	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrent
	}

	return &snmpRepository{
		pool:    pool,
		cfg:     cfg,
		sem:     make(chan struct{}, maxConcurrent),
		useWalk: useWalk,
	}
}

// createConnection creates a new SNMP connection from config
func createConnection(cfg snmpConfig) (*gosnmp.GoSNMP, error) {
	conn := &gosnmp.GoSNMP{
		Target:         cfg.target,
		Port:           cfg.port,
		Community:      cfg.community,
		Version:        cfg.version,
		Timeout:        cfg.timeout,
		Retries:        cfg.retries,
		MaxOids:        cfg.maxOids,
		MaxRepetitions: cfg.maxRepetitions,
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
	r.sem <- struct{}{}
	defer func() { <-r.sem }()

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
	r.sem <- struct{}{}
	defer func() { <-r.sem }()

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
			_ = conn.Conn.Close()
		}
	}
}

// Ping performs a lightweight reachability check by fetching sysUpTime
// (1.3.6.1.2.1.1.3.0) from the OLT. Any non-nil error is treated as "down"
// by the readiness probe.
func (r *snmpRepository) Ping() error {
	_, err := r.Get([]string{"1.3.6.1.2.1.1.3.0"})
	return err
}

// BulkWalk performs SNMP BulkWalk to get all OIDs under the given OID using GetBulk requests
func (r *snmpRepository) BulkWalk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	r.sem <- struct{}{}
	defer func() { <-r.sem }()

	conn := r.acquire()
	defer r.release(conn)

	// Some OLTs / high-latency public links don't handle GetBulk reliably — large
	// GetBulk responses get dropped/fragmented (timeout) while small GETs work.
	// Per-OLT `useWalk` (registry "walk" flag) forces the slower-but-robust
	// GetNext walk (small per-step packets) so reads complete over such links.
	var err error
	if r.useWalk {
		err = conn.Walk(oid, walkFunc)
	} else {
		err = conn.BulkWalk(oid, walkFunc)
	}
	if err != nil {
		return fmt.Errorf("SNMP walk failed: %w", err)
	}
	return nil
}
