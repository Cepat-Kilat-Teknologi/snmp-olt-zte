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

// ZTE C320 OLT Trap OID prefixes
const (
	// ZTE enterprise OID prefix
	ZTEEnterpriseOID = ".1.3.6.1.4.1.3902"

	// Common trap variable OIDs for ZTE C320
	// These are used to identify ONU events in trap PDUs
	OIDOnuIndex     = ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2" // ONU index
	OIDOnuStatus    = ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4" // ONU status
	OIDOnuOffReason = ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.7" // Offline reason
)

// ListenerConfig holds configuration for the trap listener
type ListenerConfig struct {
	Port      uint16
	Community string
	OnEvent   func(event model.TrapEvent) // callback for processed events
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

	// Parse trap variables to extract ONU information
	for _, v := range packet.Variables {
		oid := v.Name

		switch {
		case strings.HasPrefix(oid, OIDOnuStatus):
			// Extract board/PON/ONU from the OID suffix
			event.Board, event.PON, event.OnuID = parseOnuIndex(oid, OIDOnuStatus)
			if status, ok := v.Value.(int); ok {
				event.Status, event.EventType = mapStatus(status)
			}

		case strings.HasPrefix(oid, OIDOnuOffReason):
			if reason, ok := v.Value.(int); ok {
				event.EventType = mapOfflineReason(reason)
			}

		case strings.HasPrefix(oid, OIDOnuIndex):
			event.Board, event.PON, event.OnuID = parseOnuIndex(oid, OIDOnuIndex)
			if name, ok := v.Value.([]byte); ok {
				event.Description = string(name)
			} else if name, ok := v.Value.(string); ok {
				event.Description = name
			}
		}
	}

	// If we couldn't parse board/PON/ONU, try generic parsing
	if event.Board == 0 && event.PON == 0 && event.OnuID == 0 {
		event.EventType = "unknown"
		event.Description = fmt.Sprintf("Unrecognized trap from %s with %d variables", source, len(packet.Variables))
	}

	// Build description if not set
	if event.Description == "" && event.Board > 0 {
		event.Description = fmt.Sprintf("ONU %d/%d/%d %s detected", event.Board, event.PON, event.OnuID, event.EventType)
	}

	logger.Info("trap_event_processed",
		zap.String("source", source),
		zap.Int("board", event.Board),
		zap.Int("pon", event.PON),
		zap.Int("onu_id", event.OnuID),
		zap.String("event_type", event.EventType),
		zap.String("status", event.Status))

	if l.config.OnEvent != nil {
		l.config.OnEvent(event)
	}
}

// parseOnuIndex extracts board, PON, and ONU ID from an OID suffix
// OID format: prefix.boardPonEncoded.onuID
func parseOnuIndex(fullOID, prefix string) (board, pon, onuID int) {
	suffix := strings.TrimPrefix(fullOID, prefix)
	suffix = strings.TrimPrefix(suffix, ".")

	parts := strings.Split(suffix, ".")

	// The encoded value contains board and PON info
	// For ZTE C320: board 1 PON 1 base = 285278465, increment by 1 per PON
	// board 2 PON 1 base = 285278721
	encoded, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0
	}

	// Determine board and PON from encoded value
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
		// Try last part as ONU ID for simpler OID structures
		if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			onuID = id
		}
		return
	}

	// ONU ID is the next part if available
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
