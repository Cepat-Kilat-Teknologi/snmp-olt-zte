package model

// UplinkPort represents a single uplink ethernet port discovered via SNMP
// (ifName starting with "xgei_" for 10G or "gei_" for 1G).
type UplinkPort struct {
	Name        string `json:"name"`         // ifName, e.g. "xgei_1/19/1"
	Shelf       int    `json:"shelf"`        // parsed shelf number (0 if name didn't parse)
	Slot        int    `json:"slot"`         // parsed slot number (0 if name didn't parse)
	Port        int    `json:"port"`         // parsed port number (0 if name didn't parse)
	Kind        string `json:"kind"`         // "10G" (xgei_) or "1G" (gei_)
	AdminStatus string `json:"admin_status"` // "up" / "down" / numeric string
	OperStatus  string `json:"oper_status"`  // "up" / "down" / numeric string
	SpeedMbps   int    `json:"speed_mbps"`   // ifHighSpeed in Mbps (10000 for 10G, 1000 for 1G)
}

// UplinkCard represents a physical card/module discovered via ENTITY-MIB
// (only entPhysicalClass == 3 rows are considered).
type UplinkCard struct {
	EntIndex int    `json:"ent_index"` // raw entPhysical index (heuristic-free reference)
	Slot     int    `json:"slot"`      // heuristic slot: entIndex/10 - 1
	Type     string `json:"type"`      // entPhysicalDescr text
	Role     string `json:"role"`      // "gpon" / "control" / "uplink" / "power" / "other"
}

// UplinkTopology is the auto-detected OLT card + uplink-port topology returned
// by the /uplinks endpoint (Phase 1: detection only, no config writes).
type UplinkTopology struct {
	Cards []UplinkCard `json:"cards"`
	Ports []UplinkPort `json:"ports"`
}
