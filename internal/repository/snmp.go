package repository

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// SnmpRepositoryInterface is an interface that represents the SNMP repository contract
type SnmpRepositoryInterface interface {
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)       // Get SNMP data for the given OIDs
	Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error // Walk SNMP to get all OIDs under the given OID
}

// snmpRepository is a struct that implements SnmpRepositoryInterface
type snmpRepository struct {
	target    string // SNMP target IP address
	community string // SNMP community string
	port      uint16 // SNMP port number
}

// NewPonRepository is a constructor function to create a new instance of snmpRepository
func NewPonRepository(target string, community string, port uint16) SnmpRepositoryInterface {
	return &snmpRepository{ // Return a pointer to the new snmpRepository struct
		target:    target,    // SNMP target IP address
		community: community, // SNMP community string
		port:      port,      // SNMP port number
	}
}

// buildSNMPInstance for creating a new SNMP instance
func (r *snmpRepository) buildSNMPInstance() (*gosnmp.GoSNMP, error) {
	params := &gosnmp.GoSNMP{ // Initialize GoSNMP struct with parameters
		Target:    r.target,                       // SNMP target IP address
		Port:      r.port,                         // SNMP port number
		Community: r.community,                    // SNMP community string
		Version:   gosnmp.Version2c,               // SNMP version (using 2c)
		Timeout:   time.Duration(3) * time.Second, // SNMP timeout set to 3 seconds
		Retries:   1,                              // Number of retries for SNMP requests
	}

	// Set logger to nil to disable logging (default behavior of gosnmp if not set)
	// Connect creates a udp connection
	if err := params.Connect(); err != nil { // Attempt to establish connection
		return nil, fmt.Errorf("SNMP Connect error: %w", err) // Error connecting to SNMP target
	}
	return params, nil // Return the SNMP instance
}

// Get to get SNMP data for the given OIDs
func (r *snmpRepository) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	snmp, err := r.buildSNMPInstance() // Create a new SNMP instance
	if err != nil {
		return nil, err // Return error if instance creation failed
	}
	defer snmp.Conn.Close() // Defer closing the connection

	result, err := snmp.Get(oids) // Perform SNMP GET operation
	if err != nil {
		return nil, fmt.Errorf("SNMP Get failed: %w", err) // Return wrapped error on failure
	}
	return result, nil // Return result on success
}

// Walk for SNMP Walk to get all OIDs under the given OID
func (r *snmpRepository) Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	snmp, err := r.buildSNMPInstance() // Create a new SNMP instance
	if err != nil {
		return err // Return error if creation failed
	}
	defer snmp.Conn.Close() // Defer closing the connection

	err = snmp.Walk(oid, walkFunc) // Perform SNMP WALK operation with the callback function
	if err != nil {
		return fmt.Errorf("SNMP Walk failed: %w", err) // Return wrapped error on failure
	}
	return nil // Return nil on success
}
