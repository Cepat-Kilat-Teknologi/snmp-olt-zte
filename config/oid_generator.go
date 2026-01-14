package config

import "fmt"

// OID Constants for ZTE C320 OLT Device
// These are hardware-specific OIDs that follow a mathematical pattern
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

	// Board1OnuIDBase Board-PON ID Constants
	Board1OnuIDBase   = 285278464 // Actual PON 1 = 285278465 (base + 1)
	Board1OnuTypeBase = 268500992 // Actual PON 1 = 268501248 (base + 256)

	// Board2OnuIDBase Board-PON ID Constants
	Board2OnuIDBase   = 285278720 // Actual PON 1 = 285278721 (base + 1)
	Board2OnuTypeBase = 268566528 // Actual PON 1 = 268566784 (base + 256)

	// OnuIDIncrement and OnuTypeIncrement are used to calculate the actual OID suffixes
	OnuIDIncrement   = 1   // Each PON increments by 1
	OnuTypeIncrement = 256 // Each PON increments by 256
)

// GenerateBoardPonOID generates all OID configurations for a specific Board-PON combination
// using mathematical formulas instead of hardcoded config file entries.
//
// Formula:
//   - onuIDSuffix = baseOnuID + ponID
//   - onuTypeSuffix = baseOnuType + (ponID * 256)
//
// Example:
//
//	Board 1, PON 1: 285278465, 268501248
//	Board 1, PON 2: 285278466, 268501504 (+1, +256)
//	Board 2, PON 1: 285278721, 268566784
func GenerateBoardPonOID(boardID, ponID int) (*BoardPonConfig, error) {
	// Validate inputs
	if boardID < 1 || boardID > 2 {
		return nil, fmt.Errorf("invalid boardID: %d (must be 1 or 2)", boardID)
	}
	if ponID < 1 || ponID > 16 {
		return nil, fmt.Errorf("invalid ponID: %d (must be 1-16)", ponID)
	}

	// Determine base values based on board
	// Note: boardID is already validated to be 1 or 2 above
	var baseOnuID, baseOnuType int
	if boardID == 1 {
		baseOnuID = Board1OnuIDBase
		baseOnuType = Board1OnuTypeBase
	} else {
		baseOnuID = Board2OnuIDBase
		baseOnuType = Board2OnuTypeBase
	}

	// Calculate suffixes using formula
	onuIDSuffix := baseOnuID + (ponID * OnuIDIncrement)
	onuTypeSuffix := baseOnuType + (ponID * OnuTypeIncrement)

	// Generate full OIDs by concatenating prefix + suffix
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

// InitializeBoardPonMap generates all 32 Board-PON configurations dynamically.
// This replaces the need for a 20KB config file with 384 lines of OID mappings.
// Note: This function cannot fail since it always uses valid boardID (1-2) and ponID (1-16).
func InitializeBoardPonMap() (map[BoardPonKey]*BoardPonConfig, error) {
	boardPonMap := make(map[BoardPonKey]*BoardPonConfig, 32) // Pre-allocate for 32 entries (2 boards * 16 PONs)

	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			// GenerateBoardPonOID only fails for invalid boardID/ponID, which can't happen here
			cfg, _ := GenerateBoardPonOID(boardID, ponID)
			boardPonMap[BoardPonKey{BoardID: boardID, PonID: ponID}] = cfg
		}
	}

	return boardPonMap, nil
}
