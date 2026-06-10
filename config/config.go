package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Config represents the main application configuration structure
// that contains all sub-configurations for SNMP, Redis, OLT, and board/PON configs.
// The 32 individual Board{X}Pon{Y} fields have been replaced with BoardPonMap for scalability.
type Config struct { // Define the main configuration struct named Config
	SnmpCfg      SnmpConfig                      // Field to hold SNMP configuration settings
	RedisCfg     RedisConfig                     // Field to hold Redis configuration settings
	OltCfg       OltConfig                       // Field to hold OLT configuration settings
	TrapCfg      TrapConfig                      // Field to hold SNMP Trap listener configuration
	CacheCfg     CacheConfig                     // Field to hold cache TTL configuration
	Boards       []int                           // Physical GPON slots present on this OLT (sorted). C320 -> {1,2}, C300 -> e.g. {3,5}. Mirrors the default OLT in multi-OLT mode.
	BoardPons    map[int]int                     // Per-slot PON-port count (GTGO=8, GTGH=16). Slots may differ within one OLT.
	PonsPerBoard int                             // Default PON count for slots without an explicit :pons (OLT_PONS_PER_BOARD); default 16
	BoardPonMap  map[BoardPonKey]*BoardPonConfig `mapstructure:"-"` // Dynamic map keyed by (slot, pon); mirrors the default OLT in multi-OLT mode
	OLTs         []OLTRuntimeConfig              `mapstructure:"-"` // Multi-OLT registry (>=1 entry). Single entry in legacy mode.
	DefaultOLT   string                          // ID of the OLT served by the bare /board routes (back-compat)
	APIKey       string                          `mapstructure:"-"` // Legacy single API key (API_KEY); applies when APIUsers is nil
	APIUsers     map[string]APIUser              `mapstructure:"-"` // Per-tenant API-key registry (API_USERS), keyed by api_key; nil = per-user auth off
}

// SnmpConfig contains configuration parameters for SNMP connection
// including target IP address, port, and community string.
type SnmpConfig struct { // Define the SnmpConfig struct for SNMP settings
	IP            string `mapstructure:"ip"`             // IP address of the SNMP device, mapped from the "ip" configuration key
	Port          uint16 `mapstructure:"port"`           // Port number for the SNMP connection, mapped from the "port" configuration key
	Community     string `mapstructure:"community"`      // SNMP community string (password), mapped from the "community" configuration key
	MaxConcurrent int    `mapstructure:"max_concurrent"` // Maximum concurrent SNMP operations to prevent OLT saturation
}

// RedisConfig contains configuration parameters for Redis connection,
// including host, port, authentication, and connection pooling settings.
type RedisConfig struct { // Define the RedisConfig struct for Redis settings
	Host               string `mapstructure:"host"`                 // Hostname or IP address of the Redis server, mapped from "host"
	Port               string `mapstructure:"port"`                 // Port number for the Redis server, mapped from "port"
	Password           string `mapstructure:"password"`             // Password for Redis authentication, mapped from "password"
	DB                 int    `mapstructure:"db"`                   // Database index to be selected, mapped from "db"
	DefaultDB          int    `mapstructure:"default_db"`           // Default database index, mapped from "default_db"
	MinIdleConnections int    `mapstructure:"min_idle_connections"` // Minimum number of idle connections in the pool, mapped from "min_idle_connections"
	PoolSize           int    `mapstructure:"pool_size"`            // Maximum number of connections in the pool, mapped from "pool_size"
	PoolTimeout        int    `mapstructure:"pool_timeout"`         // Timeout duration for waiting for a connection from the pool, mapped from "pool_timeout"
}

// TrapConfig contains configuration parameters for the SNMP Trap listener
// including webhook notification settings for ONU events.
type TrapConfig struct {
	Enabled              bool
	Port                 uint16
	Community            string
	WebhookURL           string
	WebhookType          string // TRAP_WEBHOOK_TYPE: discord|slack|telegram|generic (auto-detect if empty)
	WebhookChatID        string // TRAP_WEBHOOK_CHAT_ID: required for Telegram
	CriticalInterval     int    // TRAP_CRITICAL_INTERVAL: batch interval for CRITICAL events (seconds)
	HighInterval         int    // TRAP_HIGH_INTERVAL: batch interval for HIGH events (seconds)
	MediumInterval       int    // TRAP_MEDIUM_INTERVAL: batch interval for MEDIUM events (seconds)
	LowInterval          int    // TRAP_LOW_INTERVAL: batch interval for LOW events (seconds)
	CriticalRepeat       int    // TRAP_CRITICAL_REPEAT: repeat interval (minutes, 0 = no repeat)
	HighRepeat           int    // TRAP_HIGH_REPEAT: repeat interval (minutes)
	MediumRepeat         int    // TRAP_MEDIUM_REPEAT: repeat interval (minutes)
	LowRepeat            int    // TRAP_LOW_REPEAT: repeat interval (minutes)
	ActionCritical       string // TRAP_ACTION_CRITICAL: action text for critical severity
	ActionHigh           string // TRAP_ACTION_HIGH: action text for high severity
	ActionMedium         string // TRAP_ACTION_MEDIUM: action text for medium severity
	ActionLow            string // TRAP_ACTION_LOW: action text for low severity
	WebhookRetries       int
	WebhookTimeout       int
	PowerMonitor         bool    // POWER_MONITOR_ENABLED
	PowerMonitorInterval int     // POWER_MONITOR_INTERVAL (seconds)
	PowerMonitorCron     string  // POWER_MONITOR_CRON (cron expression, e.g. "0 8,12,15,17,0 * * *")
	PowerMonitorTimezone string  // POWER_MONITOR_TIMEZONE (IANA timezone, e.g. "Asia/Jakarta")
	RxPowerHighThreshold float64 // RX_POWER_HIGH_THRESHOLD (dBm, overload)
	RxPowerLowThreshold  float64 // RX_POWER_LOW_THRESHOLD (dBm, weak signal)
}

// CacheConfig contains TTL configuration for Redis cache
type CacheConfig struct {
	ONUInfoTTL    int  // REDIS_ONU_INFO_TTL (seconds, default 1800 = 30min)
	ONUDetailTTL  int  // REDIS_ONU_DETAIL_TTL (seconds, default 900 = 15min)
	EmptyOnuIDTTL int  // REDIS_EMPTY_ONU_ID_TTL (seconds, default 300 = 5min)
	PreWarm       bool // CACHE_PREWARM (default true)
}

// OltConfig contains base OID configurations for OLT device management
// including common OIDs for ONU identification and type mapping.
type OltConfig struct { // Define the OltConfig struct for OLT settings
	BaseOID1        string `mapstructure:"base_oid_1"`  // First base OID string, mapped from "base_oid_1"
	BaseOID2        string `mapstructure:"base_oid_2"`  // Second base OID string, mapped from "base_oid_2"
	OnuIDNameAllPon string `mapstructure:"onu_id_name"` // OID name for ONU ID across all PONs, mapped from "onu_id_name"
	OnuTypeAllPon   string `mapstructure:"onu_type"`    // OID type for ONU across all PONs, mapped from "onu_type"
	// Timezone is the OLT's clock timezone (IANA, e.g. "Asia/Jakarta"). The OLT
	// reports last_online/last_offline in this local wall-clock, so uptime is
	// computed against it. Configurable via OLT_TIMEZONE (default Asia/Jakarta).
	Timezone string `mapstructure:"timezone"`
}

// BoardPonKey represents the unique key for board/pon lookup
type BoardPonKey struct { // Define the BoardPonKey struct to use as a map key
	BoardID int // Integer identifier for the Board
	PonID   int // Integer identifier for the PON
}

// BoardPonConfig contains OID configurations for a single Board-PON combination
// This replaces the 32 individual Board{X}Pon{Y} structs with a single reusable struct.
type BoardPonConfig struct { // Define the BoardPonConfig struct for specific Board-PON settings
	OnuIDNameOID              string `mapstructure:"onu_id_name"`               // OID for the ONU ID name, mapped from "onu_id_name"
	OnuTypeOID                string `mapstructure:"onu_type"`                  // OID for the ONU type, mapped from "onu_type"
	OnuSerialNumberOID        string `mapstructure:"onu_serial_number"`         // OID for the ONU serial number, mapped from "onu_serial_number"
	OnuRxPowerOID             string `mapstructure:"onu_rx_power"`              // OID for the ONU RX power, mapped from "onu_rx_power"
	OnuTxPowerOID             string `mapstructure:"onu_tx_power"`              // OID for the ONU TX power, mapped from "onu_tx_power"
	OnuStatusOID              string `mapstructure:"onu_status_id"`             // OID for the ONU status ID, mapped from "onu_status_id"
	OnuIPAddressOID           string `mapstructure:"onu_ip_address"`            // OID for the ONU IP address, mapped from "onu_ip_address"
	OnuDescriptionOID         string `mapstructure:"onu_description"`           // OID for the ONU description, mapped from "onu_description"
	OnuLastOnlineOID          string `mapstructure:"onu_last_online_time"`      // OID for the last online time, mapped from "onu_last_online_time"
	OnuLastOfflineOID         string `mapstructure:"onu_last_offline_time"`     // OID for the last offline time, mapped from "onu_last_offline_time"
	OnuLastOfflineReasonOID   string `mapstructure:"onu_last_offline_reason"`   // OID for the last offline reason, mapped from "onu_last_offline_reason"
	OnuGponOpticalDistanceOID string `mapstructure:"onu_gpon_optical_distance"` // OID for the GPON optical distance, mapped from "onu_gpon_optical_distance"
}

//==============================================================================
// 32 BOARD STRUCT DEFINITIONS DELETED (Board1Pon1 through Board2Pon16)
// They have been replaced by the single reusable BoardPonConfig struct above.
// This eliminates 512 lines of duplicate code and makes the system infinitely scalable.
//==============================================================================

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as int or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvAsUint16 retrieves an environment variable as uint16 or returns a default value
func getEnvAsUint16(key string, defaultValue uint16) uint16 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseUint(value, 10, 16); err == nil {
			return uint16(intVal)
		}
	}
	return defaultValue
}

// getEnvAsFloat64 retrieves an environment variable as float64 or returns a default value
func getEnvAsFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

// parseBoardSpecs parses OLT_BOARDS, which is a comma-separated list of physical
// GPON slots, each optionally annotated with its card's PON-port count as
// "slot:pons". This models a ZTE C300 whose 14 service slots can hold GTGO
// (8 PON) or GTGH (16 PON) cards in any mix:
//
//	"1,2"        -> slots 1,2 each with defaultPons
//	"3:16,5:8"   -> slot 3 GTGH (16 PON), slot 5 GTGO (8 PON)
//	"3:16,5"     -> slot 3 (16), slot 5 (defaultPons)
//
// Entries with a non-integer / out-of-range slot are skipped; a bad/out-of-range
// :pons falls back to defaultPons. When nothing valid is parsed it falls back to
// DefaultBoards (C320 slots 1-2). Returns the sorted slot list and a slot->pons map.
func parseBoardSpecs(raw string, defaultPons int) ([]int, map[int]int) {
	if defaultPons < 1 || defaultPons > MaxPonID {
		defaultPons = MaxPonID
	}
	specs := make(map[int]int)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		slotStr, ponStr, hasPon := strings.Cut(part, ":")
		slot, err := strconv.Atoi(strings.TrimSpace(slotStr))
		if err != nil || slot < 1 || slot > MaxBoardID {
			continue
		}
		pons := defaultPons
		if hasPon {
			if p, e := strconv.Atoi(strings.TrimSpace(ponStr)); e == nil && p >= 1 && p <= MaxPonID {
				pons = p
			}
		}
		specs[slot] = pons // last value wins for a duplicated slot
	}
	if len(specs) == 0 {
		for _, s := range DefaultBoards {
			specs[s] = defaultPons
		}
	}
	boards := make([]int, 0, len(specs))
	for s := range specs {
		boards = append(boards, s)
	}
	sort.Ints(boards)
	return boards, specs
}

// parseBoards returns just the sorted slot list for OLT_BOARDS (PON counts
// default to MaxPonID). Retained for callers that only need the slot set.
func parseBoards(raw string) []int {
	boards, _ := parseBoardSpecs(raw, MaxPonID)
	return boards
}

// LoadConfig loads configuration from environment variables
// All sensitive data (SNMP, Redis, Server) MUST come from environment variables
// Board/PON OID mappings are generated dynamically using mathematical formulas (no config file needed)
func LoadConfig() (*Config, error) {
	var cfg Config

	// ===================================================================
	// Load from ENVIRONMENT VARIABLES (for sensitive data)
	// ===================================================================

	// SNMP Configuration from environment (REQUIRED for production)
	cfg.SnmpCfg = SnmpConfig{
		IP:            getEnv("SNMP_HOST", ""),
		Port:          getEnvAsUint16("SNMP_PORT", 161),
		Community:     getEnv("SNMP_COMMUNITY", ""),
		MaxConcurrent: getEnvAsInt("SNMP_MAX_CONCURRENT", 5),
	}

	// Redis Configuration from environment (REQUIRED for production)
	cfg.RedisCfg = RedisConfig{
		Host:               getEnv("REDIS_HOST", "localhost"),
		Port:               getEnv("REDIS_PORT", "6379"),
		Password:           getEnv("REDIS_PASSWORD", ""),
		DB:                 getEnvAsInt("REDIS_DB", 0),
		DefaultDB:          getEnvAsInt("REDIS_DB", 0),
		MinIdleConnections: getEnvAsInt("REDIS_MIN_IDLE_CONNECTIONS", 10),
		PoolSize:           getEnvAsInt("REDIS_POOL_SIZE", 100),
		PoolTimeout:        getEnvAsInt("REDIS_POOL_TIMEOUT", 240),
	}

	// OLT Configuration - use constants or environment variables
	cfg.OltCfg = OltConfig{
		BaseOID1:        getEnv("OLT_BASE_OID_1", BaseOID1), // Fallback to constant
		BaseOID2:        getEnv("OLT_BASE_OID_2", BaseOID2), // Fallback to constant
		OnuIDNameAllPon: getEnv("ONU_ID_NAME_PREFIX", OnuIDNamePrefix),
		OnuTypeAllPon:   getEnv("ONU_TYPE_PREFIX", OnuTypePrefix),
		Timezone:        getEnv("OLT_TIMEZONE", "Asia/Jakarta"),
	}

	// SNMP Trap Configuration from environment
	cfg.TrapCfg = TrapConfig{
		Enabled:              getEnv("TRAP_ENABLED", "false") == "true",
		Port:                 getEnvAsUint16("TRAP_PORT", 1620),
		Community:            getEnv("TRAP_COMMUNITY", cfg.SnmpCfg.Community),
		WebhookURL:           getEnv("TRAP_WEBHOOK_URL", ""),
		WebhookType:          getEnv("TRAP_WEBHOOK_TYPE", ""),
		WebhookChatID:        getEnv("TRAP_WEBHOOK_CHAT_ID", ""),
		ActionCritical:       getEnv("TRAP_ACTION_CRITICAL", "Mandatory customer visit within 1x24 hours"),
		ActionHigh:           getEnv("TRAP_ACTION_HIGH", "Mandatory visit within 1x24 hours if Hard Restart does not resolve"),
		ActionMedium:         getEnv("TRAP_ACTION_MEDIUM", "Mandatory visit within 2x24 hours after notification"),
		ActionLow:            getEnv("TRAP_ACTION_LOW", "Coordinate with customer to ensure no electrical issues"),
		CriticalInterval:     getEnvAsInt("TRAP_CRITICAL_INTERVAL", 300),
		HighInterval:         getEnvAsInt("TRAP_HIGH_INTERVAL", 3600),
		MediumInterval:       getEnvAsInt("TRAP_MEDIUM_INTERVAL", 14400),
		LowInterval:          getEnvAsInt("TRAP_LOW_INTERVAL", 28800),
		CriticalRepeat:       getEnvAsInt("TRAP_CRITICAL_REPEAT", 0),
		HighRepeat:           getEnvAsInt("TRAP_HIGH_REPEAT", 0),
		MediumRepeat:         getEnvAsInt("TRAP_MEDIUM_REPEAT", 0),
		LowRepeat:            getEnvAsInt("TRAP_LOW_REPEAT", 0),
		WebhookRetries:       getEnvAsInt("TRAP_WEBHOOK_RETRIES", 3),
		WebhookTimeout:       getEnvAsInt("TRAP_WEBHOOK_TIMEOUT", 10),
		PowerMonitor:         getEnv("POWER_MONITOR_ENABLED", "false") == "true",
		PowerMonitorInterval: getEnvAsInt("POWER_MONITOR_INTERVAL", 300),
		PowerMonitorCron:     getEnv("POWER_MONITOR_CRON", ""),
		PowerMonitorTimezone: getEnv("POWER_MONITOR_TIMEZONE", ""),
		RxPowerHighThreshold: getEnvAsFloat64("RX_POWER_HIGH_THRESHOLD", -8.0),
		RxPowerLowThreshold:  getEnvAsFloat64("RX_POWER_LOW_THRESHOLD", -25.0),
	}

	// Cache TTL Configuration from environment
	cfg.CacheCfg = CacheConfig{
		ONUInfoTTL:    getEnvAsInt("REDIS_ONU_INFO_TTL", 1800),
		ONUDetailTTL:  getEnvAsInt("REDIS_ONU_DETAIL_TTL", 900),
		EmptyOnuIDTTL: getEnvAsInt("REDIS_EMPTY_ONU_ID_TTL", 300),
		PreWarm:       getEnv("CACHE_PREWARM", "true") == "true",
	}

	// ===================================================================
	// OLT slot/PON topology (multi-model: C320 + C300)
	// ===================================================================
	// OLT_BOARDS lists the physical slots that hold GPON line cards.
	//   - C320:        OLT_BOARDS=1,2          (default)
	//   - C300 sample: OLT_BOARDS=3,5
	// OLT_PONS_PER_BOARD is the PON-port count per card (8 or 16; default 16).
	cfg.PonsPerBoard = getEnvAsInt("OLT_PONS_PER_BOARD", MaxPonID)
	if cfg.PonsPerBoard < 1 || cfg.PonsPerBoard > MaxPonID {
		cfg.PonsPerBoard = MaxPonID
	}
	// OLT_BOARDS supports per-slot PON counts ("3:16,5:8") so a C300 mixing
	// GTGO (8) and GTGH (16) cards is modeled correctly; bare slots use the default.
	cfg.Boards, cfg.BoardPons = parseBoardSpecs(getEnv("OLT_BOARDS", ""), cfg.PonsPerBoard)

	// Generate Board/PON OID mappings DYNAMICALLY from the per-slot topology
	// (no config file needed). The math is slot-parametric, so the same code
	// serves C320 and C300 — only OLT_BOARDS differs.
	boardPonMap, err := InitializeBoardPonMapFromSpecs(cfg.BoardPons)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize board/pon map: %w", err)
	}
	cfg.BoardPonMap = boardPonMap

	// ===================================================================
	// Multi-OLT registry: parse OLTS (JSON), or fall back to a single OLT
	// synthesized from the legacy SNMP_* / OLT_BOARDS values above.
	// ===================================================================
	legacy := OLTRuntimeConfig{
		ID:            getEnv("DEFAULT_OLT", "default"),
		Host:          cfg.SnmpCfg.IP,
		Port:          cfg.SnmpCfg.Port,
		Community:     cfg.SnmpCfg.Community,
		MaxConcurrent: cfg.SnmpCfg.MaxConcurrent,
		Boards:        cfg.Boards,
		BoardPons:     cfg.BoardPons,
		PonsPerBoard:  cfg.PonsPerBoard,
		BoardPonMap:   cfg.BoardPonMap,
	}
	// OLTS may be supplied inline (env), via OLTS_FILE (path to a JSON file), or
	// fetched live from device-registry (REGISTRY_URL) — the canonical inventory.
	// Precedence: OLTS > OLTS_FILE > REGISTRY_URL > legacy SNMP_*. A
	// configured-but-failing REGISTRY_URL is still fail-fast (no silent wrong
	// config — snmp builds its SNMP pools once at startup with no live refresher),
	// but the fetch now retries with backoff for REGISTRY_STARTUP_TIMEOUT so a
	// cold-start race (snmp booting before device-registry is Ready) self-resolves
	// in-process instead of crash-looping the pod.
	oltsJSON, err := resolveOLTSJSON(getEnv("OLTS", ""), getEnv("OLTS_FILE", ""))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oltsJSON) == "" {
		if regURL := getEnv("REGISTRY_URL", ""); regURL != "" {
			oltsJSON, err = fetchRegistryOLTSWithRetry(regURL, getEnv("REGISTRY_API_KEY", ""))
			if err != nil {
				return nil, fmt.Errorf("REGISTRY_URL: %w", err)
			}
		}
	}
	olts, defaultOLT, err := buildOLTRegistry(oltsJSON, getEnv("DEFAULT_OLT", ""), legacy)
	if err != nil {
		return nil, err
	}
	cfg.OLTs = olts
	cfg.DefaultOLT = defaultOLT

	// Authentication. API_USERS (JSON) enables per-tenant scoping: each api_key
	// maps to a user_id + role, and OLTs are visible only to their owner (admins
	// see all). When unset, the legacy single API_KEY applies unchanged.
	cfg.APIKey = getEnv("API_KEY", "")
	apiUsers, err := buildUserRegistry(getEnv("API_USERS", ""))
	if err != nil {
		return nil, err
	}
	cfg.APIUsers = apiUsers

	// Mirror the default OLT into the legacy top-level fields so cfg-level
	// helpers (GetBoardPonConfig, ValidateConfig) reflect the default device.
	for _, o := range olts {
		if o.ID == defaultOLT {
			cfg.SnmpCfg = SnmpConfig{IP: o.Host, Port: o.Port, Community: o.Community, MaxConcurrent: o.MaxConcurrent}
			cfg.Boards = o.Boards
			cfg.BoardPons = o.BoardPons
			cfg.PonsPerBoard = o.PonsPerBoard
			cfg.BoardPonMap = o.BoardPonMap
			break
		}
	}

	return &cfg, nil
}

// BoardSet returns the configured GPON slots as a lookup set, used by HTTP
// validation to reject board_id values that are not configured on this OLT.
// Falls back to DefaultBoards when Boards is empty.
func (c *Config) BoardSet() map[int]bool {
	boards := c.Boards
	if len(boards) == 0 {
		boards = DefaultBoards
	}
	set := make(map[int]bool, len(boards))
	for _, b := range boards {
		set[b] = true
	}
	return set
}

// GetBoardPonConfig retrieves configuration for a specific board and PON
func (c *Config) GetBoardPonConfig(boardID, ponID int) (*BoardPonConfig, error) { // Define method GetBoardPonConfig on Config struct; takes boardID and ponID
	key := BoardPonKey{BoardID: boardID, PonID: ponID} // Create a BoardPonKey using the provided boardID and ponID
	cfg, ok := c.BoardPonMap[key]                      // Attempt to retrieve the configuration from the map
	if !ok {                                           // Check if the retrieval was successful (ok is false if key not found)
		return nil, fmt.Errorf("config not found for board %d, pon %d", boardID, ponID) // Return nil and a formatted error message if not found
	}
	return cfg, nil // Return the found configuration and nil error
}

// ValidateConfig validates that a BoardPonConfig exists for every (slot, pon)
// combination implied by the configured Boards x PonsPerBoard. When Boards is
// empty it falls back to DefaultBoards (slots 1-2) and a PON count of MaxPonID,
// preserving the original C320 expectation.
func (c *Config) ValidateConfig() error {
	boardPons := c.BoardPons
	if len(boardPons) == 0 {
		boardPons = map[int]int{1: MaxPonID, 2: MaxPonID}
	}

	for slot, pons := range boardPons { // Loop through all configured slots
		if pons < 1 || pons > MaxPonID {
			pons = MaxPonID
		}
		for ponID := 1; ponID <= pons; ponID++ { // Loop through that slot's PON ports
			key := BoardPonKey{BoardID: slot, PonID: ponID}
			if _, ok := c.BoardPonMap[key]; !ok {
				return fmt.Errorf("missing configuration for Board%dPon%d", slot, ponID)
			}
		}
	}
	return nil // Return nil if all validations pass
}
