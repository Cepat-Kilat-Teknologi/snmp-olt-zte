package config

import "fmt"

// OID Constants for ZTE C320 / C300 OLT devices.
//
// C300 V2.1.0 and C320 V2.1.0 share the SAME enterprise MIB tree and the SAME
// ifIndex encoding (verified live against real hardware). They differ only in
// which physical slots hold GPON line cards — C320 ships cards in slots 1-2,
// while a C300 chassis can hold them in higher slots (e.g. 3 and 5). The
// board_id in the API maps directly to the physical slot number.
//
// Two index spaces, both clean physical-slot formulas (shelf assumed 1):
//
//	ONU-ID space (BaseOID1 .3902.1082; name/serial/status/desc/rxpower/lastonline/.../distance):
//	    onuIDSuffix   = OnuIDIfIndexBase   + slot*OnuIDSlotStride   + pon*OnuIDIncrement
//	                  = 0x11010000         + slot*0x100             + pon
//	TYPE space   (BaseOID2 .3902.1012; onu type / tx power / ip address):
//	    onuTypeSuffix = OnuTypeIfIndexBase + slot*OnuTypeSlotStride + pon*OnuTypeIncrement
//	                  = 0x10000000         + slot*0x10000           + pon*0x100
//
// These reproduce the original hardcoded C320 board-1/board-2 constants exactly
// (slot 1 -> 285278465 / 268501248, slot 2 -> 285278721 / 268566784) and extend
// to any slot (C300 slot 3 -> 285278977 / 268632320, verified against TEST-ONU).
const (
	BaseOID1 = ".1.3.6.1.4.1.3902.1082"
	BaseOID2 = ".1.3.6.1.4.1.3902.1012"

	// OnuIDNamePrefix Common OID prefixes (same for all Board/PON)
	OnuIDNamePrefix              = ".500.10.2.3.3.1.2"
	OnuTypePrefix                = ".3.50.11.2.1.17"
	OnuSerialNumberPrefix        = ".500.10.2.3.3.1.18"
	OnuRxPowerPrefix             = ".500.20.2.2.2.1.10"
	OnuTxPowerPrefix             = ".3.50.12.1.1.14"
	OnuStatusIDPrefix            = ".500.10.2.3.8.1.4"
	OnuIPAddressPrefix           = ".3.50.16.1.1.10"
	OnuDescriptionPrefix         = ".500.10.2.3.3.1.3"
	OnuLastOnlineTimePrefix      = ".500.10.2.3.8.1.5"
	OnuLastOfflineTimePrefix     = ".500.10.2.3.8.1.6"
	OnuLastOfflineReasonPrefix   = ".500.10.2.3.8.1.7"
	OnuGponOpticalDistancePrefix = ".500.10.2.3.10.1.2"

	// ifIndex encoding bases and per-slot strides (see package doc above).
	// These replace the old per-board Board1*/Board2* constants with a single
	// slot-parametric formula that works for any GPON slot on C320 and C300.
	OnuIDIfIndexBase   = 285278208 // 0x11010000 — ONU-ID space (prefix 0x11, shelf 1)
	OnuIDSlotStride    = 256       // 0x100      — per-slot stride (ONU-ID space)
	OnuTypeIfIndexBase = 268435456 // 0x10000000 — TYPE space (prefix 0x10)
	OnuTypeSlotStride  = 65536     // 0x10000    — per-slot stride (TYPE space)

	// Per-PON increments within a slot.
	OnuIDIncrement   = 1   // ONU-ID space: each PON increments by 1
	OnuTypeIncrement = 256 // TYPE space: each PON increments by 256

	// MaxBoardID / MaxPonID bound the valid physical slot and PON-port range.
	// A GPON line card carries at most 16 PON ports; 30 slots comfortably covers
	// any C300/C320 chassis layout.
	MaxBoardID = 30
	MaxPonID   = 16

	// Standard-MIB OIDs for uplink/card auto-detection (IF-MIB + ENTITY-MIB).
	// Unlike the enterprise OIDs above, these are NOT board/pon-scoped — they are
	// walked whole and keyed by a trailing integer index (ifIndex for IF-MIB,
	// entity index for ENTITY-MIB). Verified live against a C300 V2.1.0; identical
	// on C320 V2.1.0. Used by GetUplinkTopology to discover cards + uplink ports
	// regardless of the physical layout (C320 SMXA xgei_1/3/x vs C300 HUVQ
	// xgei_1/19/x).
	OidIfName           = "1.3.6.1.2.1.31.1.1.1.1"   // ifName (OctetString) e.g. "xgei_1/19/1"
	OidIfAdminStatus    = "1.3.6.1.2.1.2.2.1.7"      // ifAdminStatus (INTEGER 1=up,2=down)
	OidIfOperStatus     = "1.3.6.1.2.1.2.2.1.8"      // ifOperStatus (INTEGER 1=up,2=down)
	OidIfHighSpeed      = "1.3.6.1.2.1.31.1.1.1.15"  // ifHighSpeed (Gauge32, Mbps)
	OidEntPhysicalDescr = "1.3.6.1.2.1.47.1.1.1.1.2" // entPhysicalDescr (OctetString)
	OidEntPhysicalClass = "1.3.6.1.2.1.47.1.1.1.1.5" // entPhysicalClass (INTEGER; 3=module/card)
)

// DefaultBoards is the slot set assumed when OLT_BOARDS is unset — the original
// C320 layout (line cards in slots 1 and 2). This keeps existing single-OLT
// C320 deployments behaving exactly as before.
var DefaultBoards = []int{1, 2}

// GenerateBoardPonOID generates all OID suffixes for a specific physical slot
// (boardID) and PON port (ponID) using the slot-parametric ifIndex formulas
// documented above. boardID is the physical slot number (1-2 on C320, higher on
// a C300 chassis).
func GenerateBoardPonOID(boardID, ponID int) (*BoardPonConfig, error) {
	if boardID < 1 || boardID > MaxBoardID {
		return nil, fmt.Errorf("invalid boardID: %d (must be 1-%d)", boardID, MaxBoardID)
	}
	if ponID < 1 || ponID > MaxPonID {
		return nil, fmt.Errorf("invalid ponID: %d (must be 1-%d)", ponID, MaxPonID)
	}

	// ONU-ID space suffix: name/serial/status/description/rxpower/lastonline/offline/reason/distance.
	onuIDSuffix := OnuIDIfIndexBase + boardID*OnuIDSlotStride + ponID*OnuIDIncrement
	// TYPE space suffix: onu type / tx power / ip address.
	onuTypeSuffix := OnuTypeIfIndexBase + boardID*OnuTypeSlotStride + ponID*OnuTypeIncrement

	return &BoardPonConfig{
		OnuIDNameOID:              fmt.Sprintf("%s.%d", OnuIDNamePrefix, onuIDSuffix),
		OnuTypeOID:                fmt.Sprintf("%s.%d", OnuTypePrefix, onuTypeSuffix),
		OnuSerialNumberOID:        fmt.Sprintf("%s.%d", OnuSerialNumberPrefix, onuIDSuffix),
		OnuRxPowerOID:             fmt.Sprintf("%s.%d", OnuRxPowerPrefix, onuIDSuffix),
		OnuTxPowerOID:             fmt.Sprintf("%s.%d", OnuTxPowerPrefix, onuTypeSuffix),
		OnuStatusOID:              fmt.Sprintf("%s.%d", OnuStatusIDPrefix, onuIDSuffix),
		OnuIPAddressOID:           fmt.Sprintf("%s.%d", OnuIPAddressPrefix, onuTypeSuffix),
		OnuDescriptionOID:         fmt.Sprintf("%s.%d", OnuDescriptionPrefix, onuIDSuffix),
		OnuLastOnlineOID:          fmt.Sprintf("%s.%d", OnuLastOnlineTimePrefix, onuIDSuffix),
		OnuLastOfflineOID:         fmt.Sprintf("%s.%d", OnuLastOfflineTimePrefix, onuIDSuffix),
		OnuLastOfflineReasonOID:   fmt.Sprintf("%s.%d", OnuLastOfflineReasonPrefix, onuIDSuffix),
		OnuGponOpticalDistanceOID: fmt.Sprintf("%s.%d", OnuGponOpticalDistancePrefix, onuIDSuffix),
	}, nil
}

// InitializeBoardPonMap generates BoardPonConfig entries for every (slot, pon)
// across the supplied boards with a UNIFORM ponsPerBoard. Kept for callers that
// have a single PON count; it delegates to InitializeBoardPonMapFromSpecs.
//
// When boards is empty it falls back to DefaultBoards (C320 slots 1-2); when
// ponsPerBoard is out of range it falls back to MaxPonID.
func InitializeBoardPonMap(boards []int, ponsPerBoard int) (map[BoardPonKey]*BoardPonConfig, error) {
	if len(boards) == 0 {
		boards = DefaultBoards
	}
	if ponsPerBoard < 1 || ponsPerBoard > MaxPonID {
		ponsPerBoard = MaxPonID
	}
	specs := make(map[int]int, len(boards))
	for _, slot := range boards {
		specs[slot] = ponsPerBoard
	}
	return InitializeBoardPonMapFromSpecs(specs)
}

// InitializeBoardPonMapFromSpecs generates BoardPonConfig entries for a PER-SLOT
// PON topology: boardPons maps each physical GPON slot to its card's PON-port
// count. This models a real ZTE C300 (up to 14 service slots) where slots can
// hold different cards — GTGO (8 PON) or GTGH (16 PON) — even within one OLT.
// An empty map falls back to the C320 default ({1,2} x 16).
func InitializeBoardPonMapFromSpecs(boardPons map[int]int) (map[BoardPonKey]*BoardPonConfig, error) {
	if len(boardPons) == 0 {
		boardPons = map[int]int{1: MaxPonID, 2: MaxPonID}
	}

	boardPonMap := make(map[BoardPonKey]*BoardPonConfig)
	for slot, pons := range boardPons {
		if pons < 1 || pons > MaxPonID {
			pons = MaxPonID
		}
		for pon := 1; pon <= pons; pon++ {
			cfg, err := GenerateBoardPonOID(slot, pon)
			if err != nil {
				return nil, fmt.Errorf("board %d pon %d: %w", slot, pon, err)
			}
			boardPonMap[BoardPonKey{BoardID: slot, PonID: pon}] = cfg
		}
	}
	return boardPonMap, nil
}
