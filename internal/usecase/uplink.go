package usecase

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/metrics"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
)

// uplinkNameRegex parses an uplink ifName of the form "<prefix>_<shelf>/<slot>/<port>"
// (e.g. "xgei_1/19/1" or "gei_1/3/2"). The prefix has already been stripped by
// the caller before matching, so this only matches the trailing "shelf/slot/port".
var uplinkNameRegex = regexp.MustCompile(`^(\d+)/(\d+)/(\d+)`)

// entPhysicalClassModule is the ENTITY-MIB entPhysicalClass value for a
// module/card. Only rows with this class are treated as cards.
const entPhysicalClassModule = 3

// toInt extracts an int from a gosnmp PDU value, tolerating the several integer
// representations gosnmp may return (int, uint, int64, uint64, etc.). Returns
// 0 if the value is not an integer kind.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case uint:
		return int(n)
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case uint64:
		return int(n)
	default:
		return 0
	}
}

// pduString safely extracts a string from a gosnmp OctetString PDU value.
func pduString(v interface{}) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// trailingIndex returns the trailing integer of an OID string (the last
// dot-separated element). Returns -1 if the last element is not an integer.
func trailingIndex(oid string) int {
	oid = strings.TrimSuffix(oid, ".")
	idx := strings.LastIndex(oid, ".")
	last := oid
	if idx >= 0 {
		last = oid[idx+1:]
	}
	n, err := strconv.Atoi(last)
	if err != nil {
		return -1
	}
	return n
}

// statusString maps an IF-MIB admin/oper status integer to a human label.
func statusString(v int) string {
	switch v {
	case 1:
		return "up"
	case 2:
		return "down"
	default:
		return strconv.Itoa(v)
	}
}

// cardRole classifies a card from its entPhysicalDescr (case-insensitive
// contains matching).
func cardRole(descr string) string {
	d := strings.ToLower(descr)
	switch {
	case strings.Contains(d, "gpon"):
		return "gpon"
	case strings.Contains(d, "control"):
		return "control"
	case strings.Contains(d, "ethernet interface"):
		return "uplink"
	case strings.Contains(d, "power"):
		return "power"
	default:
		return "other"
	}
}

// GetUplinkTopology auto-detects the OLT's cards and uplink ethernet ports by
// walking standard IF-MIB + ENTITY-MIB subtrees. It is read-only (Phase 1:
// detection only) and intentionally NOT cached — detection is infrequent and
// must reflect the live hardware. Singleflight still coalesces concurrent calls.
func (u *onuUsecase) GetUplinkTopology(ctx context.Context) (topo *model.UplinkTopology, err error) {
	defer metrics.RecordSNMPOperation("walk", time.Now(), &err)

	result, sgErr, _ := u.sg.Do(u.cacheKey("uplink_topology"), func() (interface{}, error) {
		logger.WithRequestID(ctx).Info("fetching_uplink_topology_snmp_walk")

		// 1) ifName: keep only uplink ports (xgei_/gei_), keyed by ifIndex.
		type portRow struct {
			name string
			kind string
		}
		ports := make(map[int]*portRow)
		walkErr := u.snmpRepository.BulkWalk(config.OidIfName, func(pdu gosnmp.SnmpPDU) error {
			idx := trailingIndex(pdu.Name)
			if idx < 0 {
				return nil
			}
			name := pduString(pdu.Value)
			switch {
			case strings.HasPrefix(name, "xgei_"):
				ports[idx] = &portRow{name: name, kind: "10G"}
			case strings.HasPrefix(name, "gei_"):
				ports[idx] = &portRow{name: name, kind: "1G"}
			}
			return nil
		})
		if walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		// 2) ifAdminStatus, ifOperStatus, ifHighSpeed — only for the uplink indices.
		admin := make(map[int]int)
		oper := make(map[int]int)
		speed := make(map[int]int)

		if walkErr = u.snmpRepository.BulkWalk(config.OidIfAdminStatus, func(pdu gosnmp.SnmpPDU) error {
			if idx := trailingIndex(pdu.Name); idx >= 0 {
				if _, ok := ports[idx]; ok {
					admin[idx] = toInt(pdu.Value)
				}
			}
			return nil
		}); walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		if walkErr = u.snmpRepository.BulkWalk(config.OidIfOperStatus, func(pdu gosnmp.SnmpPDU) error {
			if idx := trailingIndex(pdu.Name); idx >= 0 {
				if _, ok := ports[idx]; ok {
					oper[idx] = toInt(pdu.Value)
				}
			}
			return nil
		}); walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		if walkErr = u.snmpRepository.BulkWalk(config.OidIfHighSpeed, func(pdu gosnmp.SnmpPDU) error {
			if idx := trailingIndex(pdu.Name); idx >= 0 {
				if _, ok := ports[idx]; ok {
					speed[idx] = toInt(pdu.Value)
				}
			}
			return nil
		}); walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		// 3) ENTITY-MIB: class (keep class==3) + descr, keyed by entIndex.
		classByIdx := make(map[int]int)
		if walkErr = u.snmpRepository.BulkWalk(config.OidEntPhysicalClass, func(pdu gosnmp.SnmpPDU) error {
			if idx := trailingIndex(pdu.Name); idx >= 0 {
				classByIdx[idx] = toInt(pdu.Value)
			}
			return nil
		}); walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		descrByIdx := make(map[int]string)
		if walkErr = u.snmpRepository.BulkWalk(config.OidEntPhysicalDescr, func(pdu gosnmp.SnmpPDU) error {
			if idx := trailingIndex(pdu.Name); idx >= 0 {
				descrByIdx[idx] = pduString(pdu.Value)
			}
			return nil
		}); walkErr != nil {
			return nil, apperrors.NewSNMPError("walk", walkErr)
		}

		// Build ports.
		out := &model.UplinkTopology{
			Cards: make([]model.UplinkCard, 0),
			Ports: make([]model.UplinkPort, 0, len(ports)),
		}
		for idx, p := range ports {
			port := model.UplinkPort{
				Name:        p.name,
				Kind:        p.kind,
				AdminStatus: statusString(admin[idx]),
				OperStatus:  statusString(oper[idx]),
				SpeedMbps:   speed[idx],
			}
			// Parse "<prefix>_<shelf>/<slot>/<port>": strip the prefix, regex the rest.
			if u := strings.IndexByte(p.name, '_'); u >= 0 {
				if m := uplinkNameRegex.FindStringSubmatch(p.name[u+1:]); m != nil {
					port.Shelf, _ = strconv.Atoi(m[1])
					port.Slot, _ = strconv.Atoi(m[2])
					port.Port, _ = strconv.Atoi(m[3])
				}
			}
			out.Ports = append(out.Ports, port)
		}

		// Build cards (class == 3 only).
		for idx, class := range classByIdx {
			if class != entPhysicalClassModule {
				continue
			}
			descr := descrByIdx[idx]
			out.Cards = append(out.Cards, model.UplinkCard{
				EntIndex: idx,
				Slot:     idx/10 - 1, // verified heuristic: idx30->2, idx110->10, idx200->19
				Type:     descr,
				Role:     cardRole(descr),
			})
		}

		// Sort ports by slot then port; cards by slot.
		sort.Slice(out.Ports, func(i, j int) bool {
			if out.Ports[i].Slot != out.Ports[j].Slot {
				return out.Ports[i].Slot < out.Ports[j].Slot
			}
			return out.Ports[i].Port < out.Ports[j].Port
		})
		sort.Slice(out.Cards, func(i, j int) bool {
			return out.Cards[i].Slot < out.Cards[j].Slot
		})

		logger.WithRequestID(ctx).Info("uplink_topology_detected",
			zap.Int("card_count", len(out.Cards)),
			zap.Int("uplink_port_count", len(out.Ports)),
		)
		return out, nil
	})

	if sgErr != nil {
		err = sgErr
		logger.WithRequestID(ctx).Error("get_uplink_topology_failed", zap.Error(err))
		return nil, err
	}

	return result.(*model.UplinkTopology), nil
}
