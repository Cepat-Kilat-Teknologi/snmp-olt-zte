package snmp

import (
	"fmt"
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/gosnmp/gosnmp"
	"github.com/rs/zerolog/log"
)

// SetupSnmpConnection is a function to set up snmp connection
// It helps in initializing the SNMP parameters based on environment or configuration.
func SetupSnmpConnection(config *config.Config) (*gosnmp.GoSNMP, error) {
	var (
		snmpHost      string // SNMP host IP
		snmpPort      uint16 // SNMP port number
		snmpCommunity string // SNMP community string
	)

	// Check if the application is running in a development or production environment
	if os.Getenv("APP_ENV") == "development" || os.Getenv("APP_ENV") == "production" {
		// Load from environment variables
		snmpHost = os.Getenv("SNMP_HOST")
		snmpPort = utils.ConvertStringToUint16(os.Getenv("SNMP_PORT"))
		snmpCommunity = os.Getenv("SNMP_COMMUNITY")
	} else {
		// Load from config object
		snmpHost = config.SnmpCfg.IP
		snmpPort = config.SnmpCfg.Port
		snmpCommunity = config.SnmpCfg.Community
	}

	// Check if SNMP configuration is valid (non-empty)
	if snmpHost == "" || snmpPort == 0 || snmpCommunity == "" {
		log.Error().Msg("SNMP configuration is invalid")       // Log error
		return nil, fmt.Errorf("konfigurasi SNMP tidak valid") // Return error (Note: Error string is in Indonesian, keeping it as is or should I translate it? Request said English comments, didn't explicitly strict logic strings, but I will leave logic string as is to avoid breaking changes if any)
	}

	log.Info().
		Str("host", snmpHost).
		Uint16("port", snmpPort).
		Msg("Setting up SNMP connection") // Log setup information

	// Create a new SNMP target instance
	// Note: SNMP library logging is disabled, we use zerolog for application logging instead
	target := &gosnmp.GoSNMP{
		Target:    snmpHost,                       // Target IP
		Port:      snmpPort,                       // Target Port
		Community: snmpCommunity,                  // Community String
		Version:   gosnmp.Version2c,               // SNMP Version 2c
		Timeout:   time.Duration(5) * time.Second, // Timeout: 5 s (reduced from the 30s for better responsiveness)
		Retries:   2,                              // Retry count: 2 (reduced from 3, max time = 5 s × 2 = 10s)
		MaxOids:   60,                             // Maximum OIDs per request (batch size for better performance)
		Logger:    gosnmp.Logger{},                // Disable SNMP library logging (empty struct)
	}

	// Connect to the SNMP target
	err := target.Connect()
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to SNMP")      // Log connection error
		return nil, fmt.Errorf("gagal terhubung ke SNMP: %w", err) // Return wrapped error
	}

	log.Info().Msg("Successfully connected to SNMP") // Log success
	return target, nil                               // Return SNMP target object
}
