package trap

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
)

const (
	ZTEEnterpriseOID = ".1.3.6.1.4.1.3902"

	// snmpTrapOID values identifying the trap type
	OIDSnmpTrapOID   = ".1.3.6.1.6.3.1.1.4.1.0"
	OIDTrapOnuOffline = ".1.3.6.1.4.1.3902.1082.500.10.3.1.9"
	OIDTrapOnuOnline  = ".1.3.6.1.4.1.3902.1082.500.10.3.1.10"

	// ONU data OIDs carried inside the trap PDU
	OIDOnuName        = ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2"
	OIDOnuType        = ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.1"
	OIDOnuDescription = ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.3"
	OIDOnuSerial      = ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18"

	// Legacy OIDs (status/reason — may appear in some firmware versions)
	OIDOnuStatus    = ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4"
	OIDOnuOffReason = ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.7"
)

// ListenerConfig holds configuration for the trap listener
type ListenerConfig struct {
	Port      uint16
	Community string
	OnEvent   func(event model.TrapEvent)
}

// Listener wraps gosnmp.TrapListener
type Listener struct {
	tl     *gosnmp.TrapListener
	config ListenerConfig
	addr   string
}

// NewListener creates a new SNMP trap listener
func NewListener(cfg ListenerConfig) *Listener {
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)

	tl := gosnmp.NewTrapListener()
	tl.Params = &gosnmp.GoSNMP{
		Community: cfg.Community,
		Version:   gosnmp.Version2c,
	}

	listener := &Listener{
		tl:     tl,
		config: cfg,
		addr:   addr,
	}

	tl.OnNewTrap = listener.handleTrap

	return listener
}

// Start begins listening for SNMP traps (blocking)
func (l *Listener) Start() error {
	logger.Info("starting_snmp_trap_listener", zap.String("addr", l.addr))
	return l.tl.Listen(l.addr)
}

// Listening returns a channel that signals when the listener is ready
func (l *Listener) Listening() <-chan bool {
	return l.tl.Listening()
}

// Close stops the trap listener
func (l *Listener) Close() error {
	logger.Info("closing_snmp_trap_listener")
	l.tl.Close()
	return nil
}

// handleTrap processes incoming SNMP trap PDUs
func (l *Listener) handleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	source := addr.IP.String()

	logger.Debug("received_snmp_trap",
		zap.String("source", source),
		zap.Int("var_count", len(packet.Variables)))

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    source,
	}

	// Pass 1: determine event type from snmpTrapOID
	for _, v := range packet.Variables {
		if v.Name == OIDSnmpTrapOID {
			if trapOID, ok := v.Value.(string); ok {
				event.EventType, event.Status = mapTrapOID(trapOID)
			}
			break
		}
	}

	// Pass 2: extract ONU data from the remaining variables.
	// Check longer OID prefixes first to avoid collisions
	// (e.g. OIDOnuSerial ".1...1.18" vs OIDOnuType ".1...1.1").
	for _, v := range packet.Variables {
		oid := v.Name

		switch {
		case strings.HasPrefix(oid, OIDOnuSerial+"."):
			serial := extractString(v.Value)
			if idx := strings.Index(serial, ","); idx >= 0 {
				serial = serial[idx+1:]
			}
			event.SerialNumber = serial

		case strings.HasPrefix(oid, OIDOnuDescription+"."):
			event.Description = extractString(v.Value)

		case strings.HasPrefix(oid, OIDOnuName+"."):
			event.Board, event.PON, event.OnuID = parseOnuIndex(oid, OIDOnuName)
			event.Name = extractString(v.Value)

		case strings.HasPrefix(oid, OIDOnuType+"."):
			event.OnuType = extractString(v.Value)

		case strings.HasPrefix(oid, OIDOnuStatus+"."):
			if event.Board == 0 {
				event.Board, event.PON, event.OnuID = parseOnuIndex(oid, OIDOnuStatus)
			}
			if status, ok := v.Value.(int); ok {
				event.Status, event.EventType = mapStatus(status)
			}

		case strings.HasPrefix(oid, OIDOnuOffReason+"."):
			if reason, ok := v.Value.(int); ok {
				event.EventType = mapOfflineReason(reason)
			}
		}
	}

	if event.Board == 0 && event.PON == 0 && event.OnuID == 0 {
		event.EventType = "unknown"
		if event.Description == "" {
			event.Description = fmt.Sprintf("Unrecognized trap from %s with %d variables", source, len(packet.Variables))
		}
	}

	logger.Info("trap_event_processed",
		zap.String("source", source),
		zap.Int("board", event.Board),
		zap.Int("pon", event.PON),
		zap.Int("onu_id", event.OnuID),
		zap.String("event_type", event.EventType),
		zap.String("status", event.Status),
		zap.String("name", event.Name),
		zap.String("description", event.Description),
		zap.String("serial", event.SerialNumber))

	if l.config.OnEvent != nil {
		l.config.OnEvent(event)
	}
}

// extractString converts gosnmp OctetString ([]byte) or string value to string
func extractString(value interface{}) string {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return ""
	}
}

// mapTrapOID maps the snmpTrapOID value to a hint — the actual status
// must be verified via SNMP GET by the handler before alerting.
func mapTrapOID(trapOID string) (eventType, status string) {
	switch trapOID {
	case OIDTrapOnuOffline, OIDTrapOnuOnline:
		return "StatusChange", ""
	default:
		return "", ""
	}
}

// parseOnuIndex extracts board, PON, and ONU ID from an OID suffix
func parseOnuIndex(fullOID, prefix string) (board, pon, onuID int) {
	suffix := strings.TrimPrefix(fullOID, prefix)
	suffix = strings.TrimPrefix(suffix, ".")

	parts := strings.Split(suffix, ".")

	encoded, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0
	}

	const board1Base = 285278464
	const board2Base = 285278720

	switch {
	case encoded > board2Base && encoded <= board2Base+16:
		board = 2
		pon = encoded - board2Base
	case encoded > board1Base && encoded <= board1Base+16:
		board = 1
		pon = encoded - board1Base
	default:
		if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			onuID = id
		}
		return
	}

	if len(parts) >= 2 {
		if id, err := strconv.Atoi(parts[1]); err == nil {
			onuID = id
		}
	}

	return board, pon, onuID
}

// mapStatus maps SNMP status integer to human-readable status and event type
func mapStatus(status int) (statusStr, eventType string) {
	switch status {
	case 1:
		return "logging", "Logging"
	case 2:
		return "offline", "LOS"
	case 3:
		return "syncing", "Synchronization"
	case 4:
		return "online", "Online"
	case 5:
		return "offline", "DyingGasp"
	case 6:
		return "offline", "AuthFailed"
	case 7:
		return "offline", "Offline"
	default:
		return "unknown", "Unknown"
	}
}

// mapOfflineReason maps offline reason integer to event type
func mapOfflineReason(reason int) string {
	switch reason {
	case 1:
		return "Unknown"
	case 2:
		return "LOS"
	case 3:
		return "LOSi"
	case 4:
		return "LOFi"
	case 5:
		return "SFi"
	case 6:
		return "LOAi"
	case 7:
		return "LOAMi"
	case 8:
		return "AuthFail"
	case 9:
		return "PowerOff"
	case 10:
		return "DeactivateSuccess"
	case 11:
		return "DeactivateFail"
	case 12:
		return "Reboot"
	case 13:
		return "Shutdown"
	default:
		return "Unknown"
	}
}
