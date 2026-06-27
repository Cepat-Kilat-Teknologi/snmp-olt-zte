package snmp

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
)

// DefaultMaxRepetitions is the GETBULK max-repetitions applied when
// SNMP_MAX_REPETITIONS is unset. It is intentionally never 0: a GetBulk with
// max-repetitions=0 hangs on some ZTE OLTs (see snmpMaxRepetitions).
const DefaultMaxRepetitions uint32 = 20

// snmpTimeout / snmpRetries make the per-request SNMP timeout and retry count
// tunable for slow links (e.g. an OLT reached over the public internet, where
// the default 5s is too aggressive). Defaults preserve the previous behavior
// (5s, 2 retries) when the env vars are unset.
func snmpTimeout() time.Duration {
	if v := os.Getenv("SNMP_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 5 * time.Second
}

func snmpRetries() int {
	if v := os.Getenv("SNMP_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return 2
}

// snmpMaxRepetitions returns the GETBULK max-repetitions used by BulkWalk.
// gosnmp leaves MaxRepetitions at its zero value (0) unless it is set
// explicitly, and a GetBulk with max-repetitions=0 HANGS on some OLTs
// (proven live on a ZTE C300 V2.1.0: -Cr0 never returns, the request times
// out and retries, yielding empty results after ~45s, while -Cr10/-Cr50
// return rows in <0.1s). We therefore always set a sane, conservative
// default (20) and make it tunable via SNMP_MAX_REPETITIONS. A value of 0
// (or any non-positive / non-numeric value) falls back to the default so
// the hang can never be reintroduced through config.
func snmpMaxRepetitions() uint32 {
	if v := os.Getenv("SNMP_MAX_REPETITIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return uint32(n)
		}
	}
	return DefaultMaxRepetitions
}

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

	// Delegate to the explicit-parameter builder.
	return SetupSnmpConnectionWith(snmpHost, snmpPort, snmpCommunity)
}

// SetupSnmpConnectionWith builds and connects an SNMP v2c target from explicit
// parameters, performing NO environment lookup. This is what the multi-OLT
// registry uses: each OLT in the registry has its own host/port/community, so
// the env-driven SetupSnmpConnection (which always reads the single global
// SNMP_HOST) cannot be reused for more than one device.
func SetupSnmpConnectionWith(host string, port uint16, community string) (*gosnmp.GoSNMP, error) {
	// Check if SNMP configuration is valid (non-empty)
	if host == "" || port == 0 || community == "" {
		logger.Error("snmp_configuration_invalid")
		return nil, fmt.Errorf("konfigurasi SNMP tidak valid")
	}

	logger.Info("setting_up_snmp_connection",
		zap.String("host", host),
		zap.Uint16("port", port),
	)

	// Create a new SNMP target instance
	// Note: SNMP library logging is disabled; we use zap via pkg/logger for application logging instead
	target := &gosnmp.GoSNMP{
		Target:         host,                 // Target IP
		Port:           port,                 // Target Port
		Community:      community,            // Community String
		Version:        gosnmp.Version2c,     // SNMP Version 2c
		Timeout:        snmpTimeout(),        // default 5s; raise via SNMP_TIMEOUT_SECONDS for slow/public links
		Retries:        snmpRetries(),        // default 2; tune via SNMP_RETRIES
		MaxOids:        60,                   // Maximum OIDs per request (batch size for better performance)
		MaxRepetitions: snmpMaxRepetitions(), // default 20; never 0 (0 hangs GetBulk on ZTE C300) — tune via SNMP_MAX_REPETITIONS
		Logger:         gosnmp.Logger{},      // Disable SNMP library logging (empty struct)
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
