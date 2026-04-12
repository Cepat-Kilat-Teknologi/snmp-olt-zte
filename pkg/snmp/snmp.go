package snmp

import (
	"fmt"
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
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
		logger.Error("snmp_configuration_invalid")
		return nil, fmt.Errorf("konfigurasi SNMP tidak valid")
	}

	logger.Info("setting_up_snmp_connection",
		zap.String("host", snmpHost),
		zap.Uint16("port", snmpPort),
	)

	// Create a new SNMP target instance
	// Note: SNMP library logging is disabled; we use zap via pkg/logger for application logging instead
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
		logger.Error("snmp_connect_failed", zap.Error(err))
		return nil, fmt.Errorf("gagal terhubung ke SNMP: %w", err)
	}

	logger.Info("snmp_connected_successfully")
	return target, nil
}
