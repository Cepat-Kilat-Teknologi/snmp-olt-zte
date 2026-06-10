package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// resolveOLTSJSON decides the source of the OLTS registry JSON. An inline OLTS
// value wins; otherwise, when oltsFile points at a path, that file is read.
// Resolution is fail-fast: a configured-but-unreadable file (or an empty one)
// is an error rather than a silent fallback to legacy SNMP_* — a misconfigured
// network device should fail loudly at startup, not run against the wrong OLT.
// When neither source is set it returns "" so the caller uses legacy mode.
func resolveOLTSJSON(oltsInline, oltsFile string) (string, error) {
	if strings.TrimSpace(oltsInline) != "" {
		return oltsInline, nil
	}
	if strings.TrimSpace(oltsFile) == "" {
		return "", nil
	}
	data, err := os.ReadFile(oltsFile)
	if err != nil {
		return "", fmt.Errorf("OLTS_FILE %q: %w", oltsFile, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("OLTS_FILE %q is empty", oltsFile)
	}
	return string(data), nil
}

// OLTRuntimeConfig is one OLT in the multi-OLT registry: its identity, SNMP
// connection parameters, and the GPON slot/PON topology (plus the resulting
// per-OLT OID map). One instance can serve many of these — C320 and C300
// side by side — because the OID encoding is identical and only the populated
// slots (Boards) differ.
type OLTRuntimeConfig struct {
	ID            string
	UserID        int // owner (tenant) id; 0 = unowned (admin-only when API_USERS is set)
	Host          string
	Port          uint16
	Community     string
	MaxConcurrent int
	Boards        []int       // sorted GPON slot list
	BoardPons     map[int]int // per-slot PON count (GTGO=8, GTGH=16)
	PonsPerBoard  int         // default PON count for bare slots
	BoardPonMap   map[BoardPonKey]*BoardPonConfig
	// UseWalk forces GetNext (Walk) instead of GetBulk for this OLT. Set it for
	// OLTs reached over a lossy/high-latency link (e.g. the public internet)
	// where large GetBulk responses get dropped. LAN OLTs leave it false (fast).
	UseWalk bool
}

// oltJSON is the wire shape of one entry in the OLTS env var (a JSON array).
type oltJSON struct {
	ID            string `json:"id"`
	UserID        int    `json:"user_id"` // owner (tenant) id; 0/omitted = unowned
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Community     string `json:"community"`
	MaxConcurrent int    `json:"maxConcurrent"`
	Boards        string `json:"boards"` // CSV slots, optionally slot:pons, e.g. "3:16,5:8"
	PonsPerBoard  int    `json:"ponsPerBoard"`
	Walk          bool   `json:"walk"` // use GetNext walk (robust over slow/public links)
}

// oltIDPattern restricts OLT ids to URL-path-safe characters (they appear in
// /api/v1/olt/{id}/... and in Redis cache key prefixes).
var oltIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// buildOLTRegistry constructs the OLT registry. When oltsJSON is non-empty it is
// parsed as a JSON array (multi-OLT mode); otherwise a single OLT is synthesized
// from the supplied legacy descriptor (back-compat with SNMP_* / OLT_BOARDS).
// It returns the registry, the default OLT id (served by the bare /board routes),
// and any validation error.
func buildOLTRegistry(oltsJSON, defaultOLTEnv string, legacy OLTRuntimeConfig) ([]OLTRuntimeConfig, string, error) {
	var olts []OLTRuntimeConfig

	if oltsJSON == "" {
		// Legacy single-OLT mode: SNMP_HOST / SNMP_COMMUNITY are required here
		// (set OLTS instead to run multiple OLTs in one instance).
		if legacy.Host == "" {
			return nil, "", fmt.Errorf("SNMP_HOST environment variable is required")
		}
		if legacy.Community == "" {
			return nil, "", fmt.Errorf("SNMP_COMMUNITY environment variable is required")
		}
		if legacy.ID == "" {
			legacy.ID = "default"
		}
		olts = []OLTRuntimeConfig{legacy}
	} else {
		var entries []oltJSON
		if err := json.Unmarshal([]byte(oltsJSON), &entries); err != nil {
			return nil, "", fmt.Errorf("invalid OLTS JSON: %w", err)
		}
		if len(entries) == 0 {
			return nil, "", fmt.Errorf("OLTS must contain at least one OLT")
		}
		for i, e := range entries {
			o, err := oltFromJSON(e, legacy.MaxConcurrent)
			if err != nil {
				return nil, "", fmt.Errorf("OLTS[%d]: %w", i, err)
			}
			olts = append(olts, o)
		}
	}

	// Validate unique, URL-safe ids.
	seen := make(map[string]bool, len(olts))
	for _, o := range olts {
		if !oltIDPattern.MatchString(o.ID) {
			return nil, "", fmt.Errorf("OLT id %q is invalid (allowed: letters, digits, '-', '_')", o.ID)
		}
		if seen[o.ID] {
			return nil, "", fmt.Errorf("duplicate OLT id %q", o.ID)
		}
		seen[o.ID] = true
	}

	defaultOLT := defaultOLTEnv
	if defaultOLT == "" {
		defaultOLT = olts[0].ID
	}
	if !seen[defaultOLT] {
		return nil, "", fmt.Errorf("DEFAULT_OLT %q is not one of the configured OLTs", defaultOLT)
	}

	return olts, defaultOLT, nil
}

// oltFromJSON validates and normalizes one OLTS entry, applying defaults and
// generating its BoardPonMap.
func oltFromJSON(e oltJSON, defaultMaxConcurrent int) (OLTRuntimeConfig, error) {
	if e.ID == "" {
		return OLTRuntimeConfig{}, fmt.Errorf("id is required")
	}
	if e.Host == "" {
		return OLTRuntimeConfig{}, fmt.Errorf("host is required (id=%s)", e.ID)
	}
	if e.Community == "" {
		return OLTRuntimeConfig{}, fmt.Errorf("community is required (id=%s)", e.ID)
	}

	port := uint16(e.Port)
	if port == 0 {
		port = 161
	}
	maxConc := e.MaxConcurrent
	if maxConc < 1 {
		maxConc = defaultMaxConcurrent
	}
	if maxConc < 1 {
		maxConc = 5
	}
	defaultPons := e.PonsPerBoard
	if defaultPons < 1 || defaultPons > MaxPonID {
		defaultPons = MaxPonID
	}
	// "boards" supports per-slot PON counts, e.g. "3:16,5:8" (GTGH + GTGO).
	boards, boardPons := parseBoardSpecs(e.Boards, defaultPons)
	bpMap, err := InitializeBoardPonMapFromSpecs(boardPons)
	if err != nil {
		return OLTRuntimeConfig{}, fmt.Errorf("id=%s: %w", e.ID, err)
	}

	return OLTRuntimeConfig{
		ID:            e.ID,
		UserID:        e.UserID,
		Host:          e.Host,
		Port:          port,
		Community:     e.Community,
		MaxConcurrent: maxConc,
		Boards:        boards,
		BoardPons:     boardPons,
		PonsPerBoard:  defaultPons,
		BoardPonMap:   bpMap,
		UseWalk:       e.Walk,
	}, nil
}

// ForOLT returns a *Config view scoped to a single OLT: the shared sub-configs
// (OltCfg base OIDs, CacheCfg, RedisCfg) are preserved, while SnmpCfg, Boards,
// PonsPerBoard and BoardPonMap reflect that OLT. The per-OLT usecase is built
// from this so its OID generation and board lookups target the right device.
func (c *Config) ForOLT(o OLTRuntimeConfig) *Config {
	clone := *c
	clone.SnmpCfg = SnmpConfig{IP: o.Host, Port: o.Port, Community: o.Community, MaxConcurrent: o.MaxConcurrent}
	clone.Boards = o.Boards
	clone.BoardPons = o.BoardPons
	clone.PonsPerBoard = o.PonsPerBoard
	clone.BoardPonMap = o.BoardPonMap
	clone.OLTs = nil
	return &clone
}
