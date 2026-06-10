package usecase

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/gosnmp/gosnmp"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Constants for ONU configuration limits and cache TTL values
const (
	// MaxOnuIDPerPon is the maximum number of ONU IDs allowed per PON port
	MaxOnuIDPerPon = 128

	// TimezoneOffsetWIB is the timezone offset for WIB (Western Indonesian Time / UTC+7)
	// This offset is used to adjust SNMP timestamps which are typically in UTC
	// to the local timezone (Asia/Jakarta - WIB)
	TimezoneOffsetWIB = 7 * time.Hour

	// SNMPOIDSuffix is the default suffix used for SNMP OID queries
	// that require an additional index (e.g., for power readings, IP address)
	SNMPOIDSuffix = ".1"

	// DateTimeFormat is the standard date-time format used for parsing
	// SNMP timestamps (format: YYYY-MM-DD HH:MM:SS)
	DateTimeFormat = "2006-01-02 15:04:05"
)

// Redis key type constants for consistent key generation
const (
	RedisKeyTypeONUInfo    = "onu_info"
	RedisKeyTypeEmptyOnuID = "empty_onu_id"
	RedisKeyTypeONUDetail  = "onu_detail"
)

// GenerateRedisKey creates a consistent Redis key based on key type, board ID, and PON ID
func GenerateRedisKey(keyType string, boardID, ponID int) string {
	switch keyType {
	case RedisKeyTypeEmptyOnuID:
		return fmt.Sprintf("board_%d_pon_%d_empty_onu_id", boardID, ponID)
	default:
		return fmt.Sprintf("board_%d_pon_%d", boardID, ponID)
	}
}

// GenerateONUDetailRedisKey creates a Redis key for ONU detail cache
func GenerateONUDetailRedisKey(boardID, ponID, onuID int) string {
	return fmt.Sprintf("board_%d_pon_%d_onu_%d_detail", boardID, ponID, onuID)
}

// OnuUseCaseInterface is an interface that represents the auth's usecase contract
type OnuUseCaseInterface interface {
	GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)                             // Get ONU info by board and PON
	GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error)                   // Get specific ONU info
	GetEmptyOnuID(ctx context.Context, boardID, ponID int) ([]model.OnuID, error)                                              // Get empty ONU IDs
	GetOnuIDAndSerialNumber(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error)                          // Get ONU IDs and serial numbers
	UpdateEmptyOnuID(ctx context.Context, boardID, ponID int) error                                                            // Update empty ONU IDs cache
	GetByBoardIDAndPonIDWithPagination(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) // Get paginated ONU info
	DeleteCache(ctx context.Context, boardID, ponID int) error                                                                 // Delete cache for specific board/pon
	InvalidateONUCache(ctx context.Context, boardID, ponID, onuID int) error                                                   // Invalidate ONU detail + board/pon cache for fresh SNMP query
	GetUplinkTopology(ctx context.Context) (*model.UplinkTopology, error)                                                      // Auto-detect cards + uplink ethernet ports via standard MIBs (read-only)
	PreWarmCache(ctx context.Context)
}

// onuUsecase represent the auth's usecase
type onuUsecase struct {
	snmpRepository  repository.SnmpRepositoryInterface     // SNMP repository dependency
	redisRepository repository.OnuRedisRepositoryInterface // Redis repository dependency
	cfg             *config.Config                         // Configuration dependency
	oltID           string                                 // OLT identifier for Redis cache namespacing (empty = no prefix, legacy/default OLT)
	sg              singleflight.Group                     // Singleflight group for request coalescing (per-OLT instance, so keys need no OLT prefix)
}

// cacheKey namespaces a Redis cache key with the OLT id so that multiple OLTs
// sharing one Redis instance never collide (e.g. board 1/pon 1 on two devices).
// An empty oltID returns the key unchanged — preserving legacy single-OLT keys.
func (u *onuUsecase) cacheKey(base string) string {
	if u.oltID == "" {
		return base
	}
	return "olt_" + u.oltID + "_" + base
}

// NewOnuUsecase will create an object that represents the auth usecase.
// It uses no cache-key namespace (legacy single-OLT behavior).
func NewOnuUsecase(
	snmpRepository repository.SnmpRepositoryInterface, redisRepository repository.OnuRedisRepositoryInterface,
	cfg *config.Config,
) OnuUseCaseInterface {
	return NewOnuUsecaseForOLT(snmpRepository, redisRepository, cfg, "")
}

// NewOnuUsecaseForOLT creates a usecase bound to a specific OLT id, used by the
// multi-OLT registry. The oltID namespaces this OLT's Redis cache keys; pass ""
// for the default/legacy OLT to keep keys unprefixed.
func NewOnuUsecaseForOLT(
	snmpRepository repository.SnmpRepositoryInterface, redisRepository repository.OnuRedisRepositoryInterface,
	cfg *config.Config, oltID string,
) OnuUseCaseInterface {
	return &onuUsecase{
		snmpRepository:  snmpRepository,       // Inject SNMP repository
		redisRepository: redisRepository,      // Inject Redis repository
		cfg:             cfg,                  // Inject configuration
		oltID:           oltID,                // Cache-key namespace
		sg:              singleflight.Group{}, // Initialize a singleflight group
	}
}

// getOltInfo is a function to get OLT information
func (u *onuUsecase) getOltConfig(boardID, ponID int) (*model.OltConfig, error) {
	cfg, err := u.getBoardConfig(boardID, ponID) // Retrieve board configuration
	if err != nil {
		logger.Error("olt_config_failed", zap.Error(err)) // Log error
		return nil, err
	}
	return cfg, nil // Return config
}

// getBoardConfig is a function to get board configuration
// Refactored to use dynamic config lookup instead of massive switch statements
func (u *onuUsecase) getBoardConfig(boardID, ponID int) (*model.OltConfig, error) {
	// Get board-PON specific config from a map
	ponCfg, err := u.cfg.GetBoardPonConfig(boardID, ponID) // Look up config from the loaded configuration map
	if err != nil {
		return nil, apperrors.NewConfigError("invalid board/pon combination", err) // Return config error
	}

	// Determine base OID based on boardID
	baseOID := u.cfg.OltCfg.BaseOID1 // Retrieve base OID from global config

	// Build OltConfig from dynamic config
	return &model.OltConfig{
		BaseOID:                   baseOID,                          // Set Base OID
		OnuIDNameOID:              ponCfg.OnuIDNameOID,              // Set ONU ID Name OID
		OnuTypeOID:                ponCfg.OnuTypeOID,                // Set ONU Type OID
		OnuSerialNumberOID:        ponCfg.OnuSerialNumberOID,        // Set ONU Serial Number OID
		OnuRxPowerOID:             ponCfg.OnuRxPowerOID,             // Set ONU RX Power OID
		OnuTxPowerOID:             ponCfg.OnuTxPowerOID,             // Set ONU TX Power OID
		OnuStatusOID:              ponCfg.OnuStatusOID,              // Set ONU Status OID
		OnuIPAddressOID:           ponCfg.OnuIPAddressOID,           // Set ONU IP Address OID
		OnuDescriptionOID:         ponCfg.OnuDescriptionOID,         // Set ONU Description OID
		OnuLastOnlineOID:          ponCfg.OnuLastOnlineOID,          // Set ONU Last Online OID
		OnuLastOfflineOID:         ponCfg.OnuLastOfflineOID,         // Set ONU Last Offline OID
		OnuLastOfflineReasonOID:   ponCfg.OnuLastOfflineReasonOID,   // Set ONU Last Offline Reason OID
		OnuGponOpticalDistanceOID: ponCfg.OnuGponOpticalDistanceOID, // Set ONU GPON Optical Distance OID
	}, nil
}

func (u *onuUsecase) GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
	logger.WithRequestID(ctx).Info("getting_all_onu_information",
		zap.Int("board_id", boardID),
		zap.Int("pon_id", ponID),
	)

	key := fmt.Sprintf("onuinfo-b%d-p%d", boardID, ponID) // Create unique key for singleflight

	// Using simple flight to prevent duplicate SNMP requests
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config
		oltConfig, err := u.getOltConfig(boardID, ponID) // Get OLT config based on Board ID and PON ID
		if err != nil {
			logger.WithRequestID(ctx).Error("olt_config_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Redis key using helper function
		redisKey := u.cacheKey(GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID))

		// Check if data is already cached in Redis
		cachedOnuData, err := u.redisRepository.GetONUInfoList(ctx, redisKey) // Get ONU Information from Redis
		if err == nil && cachedOnuData != nil {
			logger.WithRequestID(ctx).Info("onu_info_cache_hit", zap.String("redis_key", redisKey))

			// Background refresh if TTL is low
			ttl, ttlErr := u.redisRepository.GetTTL(ctx, redisKey)
			if ttlErr == nil && ttl > 0 && ttl < time.Duration(u.cfg.CacheCfg.ONUInfoTTL/5)*time.Second {
				go func() {
					logger.Info("background_cache_refresh_triggered", zap.String("redis_key", redisKey))
					u.refreshONUInfoCache(context.Background(), boardID, ponID, oltConfig, redisKey)
				}()
			}

			return cachedOnuData, nil
		}

		// SNMP Walk to get Information from OLT Board and PON
		logger.WithRequestID(ctx).Info("fetching_onu_information_snmp_walk", zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
		// Create a map to store SNMP Walk results
		snmpDataMap := make(map[string]gosnmp.SnmpPDU)
		// Perform SNMP Walk to get ONU ID and Name using snmpRepository Walk method with timeout context parameter
		err = u.snmpRepository.BulkWalk(oltConfig.BaseOID+oltConfig.OnuIDNameOID, func(pdu gosnmp.SnmpPDU) error {
			snmpDataMap[utils.ExtractONUID(pdu.Name)] = pdu // Store PDU in map with ONU ID as key
			return nil
		})

		if err != nil {
			return nil, err // Return error if walkthrough fails
		}

		var onuInformationList []model.ONUInfoPerBoard // Create a slice of ONUInfoPerBoard

		// Loop through an SNMP data map to get ONU information based on ONU ID and ONU Name stored in a map before and store
		for _, pdu := range snmpDataMap {
			// Create a new ONUInfoPerBoard struct and populate it with ONU ID, ONU Name, ONU Type, ONU Serial Number, ONU RX Power, ONU Status
			onuInfo := model.ONUInfoPerBoard{
				Board: boardID,                        // Set Board ID
				PON:   ponID,                          // Set PON ID
				ID:    utils.ExtractIDOnuID(pdu.Name), // Extract and set ONU ID
				Name:  utils.ExtractName(pdu.Value),   // Extract and set ONU Name
			}

			// Batch SNMP Get: fetch type, serial, rx_power, status in one request
			onuIDStr := strconv.Itoa(onuInfo.ID)
			batchOIDs := []string{
				u.cfg.OltCfg.BaseOID2 + oltConfig.OnuTypeOID + "." + onuIDStr,
				u.cfg.OltCfg.BaseOID1 + oltConfig.OnuSerialNumberOID + "." + onuIDStr,
				u.cfg.OltCfg.BaseOID1 + oltConfig.OnuRxPowerOID + "." + onuIDStr + SNMPOIDSuffix,
				u.cfg.OltCfg.BaseOID1 + oltConfig.OnuStatusOID + "." + onuIDStr,
			}

			batchResult, err := u.snmpRepository.Get(batchOIDs)
			if err == nil && len(batchResult.Variables) >= 4 {
				onuInfo.OnuType = utils.ExtractName(batchResult.Variables[0].Value)
				onuInfo.SerialNumber = utils.ExtractSerialNumber(batchResult.Variables[1].Value)
				if power, convErr := utils.ConvertAndMultiply(batchResult.Variables[2].Value); convErr == nil {
					onuInfo.RXPower = power
				}
				onuInfo.Status = utils.ExtractAndGetStatus(batchResult.Variables[3].Value)
			}

			// Add info to the list
			onuInformationList = append(onuInformationList, onuInfo)
		}

		// Sort the ONU information list by ID
		sort.Slice(onuInformationList, func(i, j int) bool {
			return onuInformationList[i].ID < onuInformationList[j].ID
		})

		// Save the ONU information list to Redis with a 10-minute expiration time
		// Balanced: 600s (10min) - fresh enough while maintaining a good cache hit rate.
		// Detached context: the walk is done, so don't abort the cache write if the
		// original caller went away (see the serial-list save for the full rationale).
		saveCtx, cancelSave := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err = u.redisRepository.SaveONUInfoList(saveCtx, redisKey, u.cfg.CacheCfg.ONUInfoTTL, onuInformationList)
		cancelSave()
		if err != nil {
			logger.WithRequestID(ctx).Error("redis_save_onu_info_failed", zap.Error(err), zap.String("redis_key", redisKey))
		} else {
			logger.WithRequestID(ctx).Info("onu_info_saved_to_redis", zap.String("redis_key", redisKey))
		}

		// Return the ONU information list
		return onuInformationList, nil
	})

	if err != nil {
		logger.WithRequestID(ctx).Error("get_onu_information_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
		return nil, err
	}

	return result.([]model.ONUInfoPerBoard), nil // Return the result from the cache or SNMP Walk
}

// refreshONUInfoCache performs a background SNMP fetch and updates the Redis cache
func (u *onuUsecase) refreshONUInfoCache(ctx context.Context, boardID, ponID int, oltConfig *model.OltConfig, redisKey string) {
	snmpDataMap := make(map[string]gosnmp.SnmpPDU)
	err := u.snmpRepository.BulkWalk(oltConfig.BaseOID+oltConfig.OnuIDNameOID, func(pdu gosnmp.SnmpPDU) error {
		snmpDataMap[utils.ExtractONUID(pdu.Name)] = pdu
		return nil
	})
	if err != nil {
		logger.Error("background_refresh_snmp_walk_failed", zap.Error(err))
		return
	}

	var list []model.ONUInfoPerBoard
	for _, pdu := range snmpDataMap {
		onuInfo := model.ONUInfoPerBoard{
			Board: boardID, PON: ponID,
			ID:   utils.ExtractIDOnuID(pdu.Name),
			Name: utils.ExtractName(pdu.Value),
		}
		onuIDStr := strconv.Itoa(onuInfo.ID)
		batchOIDs := []string{
			u.cfg.OltCfg.BaseOID2 + oltConfig.OnuTypeOID + "." + onuIDStr,
			u.cfg.OltCfg.BaseOID1 + oltConfig.OnuSerialNumberOID + "." + onuIDStr,
			u.cfg.OltCfg.BaseOID1 + oltConfig.OnuRxPowerOID + "." + onuIDStr + SNMPOIDSuffix,
			u.cfg.OltCfg.BaseOID1 + oltConfig.OnuStatusOID + "." + onuIDStr,
		}
		batchResult, err := u.snmpRepository.Get(batchOIDs)
		if err == nil && len(batchResult.Variables) >= 4 {
			onuInfo.OnuType = utils.ExtractName(batchResult.Variables[0].Value)
			onuInfo.SerialNumber = utils.ExtractSerialNumber(batchResult.Variables[1].Value)
			if power, convErr := utils.ConvertAndMultiply(batchResult.Variables[2].Value); convErr == nil {
				onuInfo.RXPower = power
			}
			onuInfo.Status = utils.ExtractAndGetStatus(batchResult.Variables[3].Value)
		}
		list = append(list, onuInfo)
	}

	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

	if err := u.redisRepository.SaveONUInfoList(ctx, redisKey, u.cfg.CacheCfg.ONUInfoTTL, list); err != nil {
		logger.Error("background_refresh_redis_save_failed", zap.Error(err))
	} else {
		logger.Info("background_refresh_cache_updated", zap.String("redis_key", redisKey))
	}
}

func (u *onuUsecase) GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (
	model.ONUCustomerInfo, error,
) {
	// Set key for simple flight
	key := fmt.Sprintf("onu:%d:%d:%d", boardID, ponID, onuID)

	// Using simple flight to prevent duplicate SNMP requests
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		oltConfig, err := u.getOltConfig(boardID, ponID) // Get OLT config based on Board ID and PON ID
		if err != nil {
			logger.WithRequestID(ctx).Error("olt_config_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return model.ONUCustomerInfo{}, err
		}

		// ONU detail is intentionally CACHE-FREE: always perform a live SNMP read
		// so the modal reflects the device in real time (right after a restart,
		// re-provision, power change, etc.) instead of a stale or partial
		// snapshot. The board/PON LIST stays cached for fast browsing — only this
		// per-ONU detail bypasses the cache. Singleflight (above) still coalesces
		// concurrent identical opens into one read.

		var onuInformationList model.ONUCustomerInfo   // Create a variable to store ONU information
		snmpDataMap := make(map[string]gosnmp.SnmpPDU) // Create a map to store SNMP Walk results

		logger.WithRequestID(ctx).Info("fetching_onu_detail_snmp_walk",
			zap.Int("board_id", boardID),
			zap.Int("pon_id", ponID),
			zap.Int("onu_id", onuID),
		)

		// Get ONU ID and Name using snmpRepository Walk method with timeout context parameter
		err = u.snmpRepository.BulkWalk(oltConfig.BaseOID+oltConfig.OnuIDNameOID+"."+strconv.Itoa(onuID),
			func(pdu gosnmp.SnmpPDU) error {
				snmpDataMap[utils.ExtractONUID(pdu.Name)] = pdu // Extract ID and store PDU
				return nil
			})
		if err != nil {
			logger.WithRequestID(ctx).Error("snmp_walk_failed", zap.Error(err), zap.String("oid", oltConfig.BaseOID+oltConfig.OnuIDNameOID))
			return model.ONUCustomerInfo{}, apperrors.NewSNMPError("Walk", err) // Return SNMP error
		}

		// Loop through an SNMP data map to get ONU information based on ONU ID and ONU Name stored in a map before and store
		for _, pdu := range snmpDataMap {
			// Create a new ONUInfoPerBoard struct and populate it with ONU ID, ONU Name, ONU Type, ONU Serial Number, ONU RX Power, ONU Status
			onuInfo := model.ONUCustomerInfo{
				Board: boardID,                        // Set board ID
				PON:   ponID,                          // Set PON ID
				ID:    utils.ExtractIDOnuID(pdu.Name), // Extract ID
				Name:  utils.ExtractName(pdu.Value),   // Extract Name
			}

			// Sequential SNMP Gets (gosnmp is not thread-safe, parallel caused worse performance)
			// Get Data ONU Type from SNMP Walk using the getONUType method
			if onuType, err := u.getONUType(oltConfig.OnuTypeOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.OnuType = onuType
			}

			// Get Data ONU Serial Number from SNMP Walk using the getSerialNumber method
			if serial, err := u.getSerialNumber(oltConfig.OnuSerialNumberOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.SerialNumber = serial
			}

			// Get Data ONU RX Power from SNMP Walk using the getRxPower method
			if rx, err := u.getRxPower(oltConfig.OnuRxPowerOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.RXPower = rx
			}

			// Get Data ONU TX Power from SNMP Walk using getTxPower method
			if tx, err := u.getTxPower(oltConfig.OnuTxPowerOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.TXPower = tx
			}

			// Get Data ONU Status from SNMP Walk using getStatus method
			if status, err := u.getStatus(oltConfig.OnuStatusOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.Status = status
			}

			// Get Data ONU IP Address from SNMP Walk using the getIPAddress method
			if ip, err := u.getIPAddress(oltConfig.OnuIPAddressOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.IPAddress = ip
			}

			// Get Data ONU Description from SNMP Walk using the getDescription method
			if desc, err := u.getDescription(oltConfig.OnuDescriptionOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.Description = desc
			}

			// Get Data ONU Last Online from SNMP Walk using the getLastOnline method
			if lastOnline, err := u.getLastOnline(oltConfig.OnuLastOnlineOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.LastOnline = lastOnline
			}

			// Get Data ONU Last Offline from SNMP Walk using the getLastOffline method
			if lastOffline, err := u.getLastOffline(oltConfig.OnuLastOfflineOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.LastOffline = lastOffline
			}

			// Get Data ONU Last Offline Reason from SNMP Walk using the getLastOfflineReason method
			if uptime, err := u.getUptimeDuration(onuInfo.LastOnline); err == nil {
				onuInfo.Uptime = uptime
			}

			// Get Data ONU Last Downtime Duration from SNMP Walk using the getLastDownDuration method
			if downtime, err := u.getLastDownDuration(onuInfo.LastOffline, onuInfo.LastOnline); err == nil {
				onuInfo.LastDownTimeDuration = downtime
			}

			// Get Data ONU Last Offline Reason from SNMP Walk using the getLastOfflineReason method
			if reason, err := u.getLastOfflineReason(oltConfig.OnuLastOfflineReasonOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.LastOfflineReason = reason
			}

			// Get Data ONU GPON Optical Distance from SNMP Walk using getOnuGponOpticalDistance method
			if dist, err := u.getOnuGponOpticalDistance(oltConfig.OnuGponOpticalDistanceOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.GponOpticalDistance = dist
			}

			onuInformationList = onuInfo // Append ONU information to the onuInformationList
		}

		// No cache write: detail is always served live (see note above).
		return onuInformationList, nil // Return the ONU information list
	})

	if err != nil {
		return model.ONUCustomerInfo{}, err // Return error
	}

	return result.(model.ONUCustomerInfo), nil // Return the result from the cache or SNMP Walk
}

func (u *onuUsecase) GetEmptyOnuID(ctx context.Context, boardID, ponID int) ([]model.OnuID, error) {
	// Set key for simple flight
	key := fmt.Sprintf("empty_onu_id:%d:%d", boardID, ponID)

	// Using simple flight to prevent duplicate requests for the same data
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config based on Board ID and PON ID
		oltConfig, err := u.getOltConfig(boardID, ponID)
		if err != nil {
			logger.WithRequestID(ctx).Error("olt_config_failed_empty_onu_id", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Redis Key using helper function
		redisKey := u.cacheKey(GenerateRedisKey(RedisKeyTypeEmptyOnuID, boardID, ponID))

		// Try to get data from Redis using the GetOnuIDCtx method with context and Redis key as a parameter
		cachedOnuData, err := u.redisRepository.GetOnuIDCtx(ctx, redisKey)
		if err == nil && cachedOnuData != nil {
			logger.WithRequestID(ctx).Info("empty_onu_ids_cache_hit", zap.String("redis_key", redisKey))
			// If data exists in Redis, return data from Redis
			return cachedOnuData, nil
		}

		// Perform SNMP Walk to get ONU ID and ONU Name
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		emptyOnuIDList := make([]model.OnuID, 0) // Initialize an empty list

		logger.WithRequestID(ctx).Info("fetching_empty_onu_ids_snmp_walk", zap.Int("board_id", boardID), zap.Int("pon_id", ponID))

		// Perform SNMP Walk to get ONU ID and Name
		err = u.snmpRepository.BulkWalk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			idOnuID := utils.ExtractIDOnuID(pdu.Name) // Extract ID
			emptyOnuIDList = append(emptyOnuIDList, model.OnuID{
				Board: boardID,
				PON:   ponID,
				ID:    idOnuID,
			})
			return nil
		})
		if err != nil {
			logger.WithRequestID(ctx).Error("snmp_walk_empty_onu_ids_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Create a map to store numbers to be deleted
		numbersToRemove := make(map[int]bool)

		for _, onuInfo := range emptyOnuIDList {
			numbersToRemove[onuInfo.ID] = true // Mark ID as existing
		}

		// Remove the numbers that should not be added to the emptyOnuIDList
		emptyOnuIDList = emptyOnuIDList[:0]

		// Loop through MaxOnuIDPerPon numbers to find empty ONU IDs
		for i := 1; i <= MaxOnuIDPerPon; i++ {
			if _, ok := numbersToRemove[i]; !ok { // If ID is not in existing IDs, it's empty
				emptyOnuIDList = append(emptyOnuIDList, model.OnuID{
					Board: boardID,
					PON:   ponID,
					ID:    i,
				})
			}
		}

		// Sort by ID ascending
		sort.Slice(emptyOnuIDList, func(i, j int) bool {
			return emptyOnuIDList[i].ID < emptyOnuIDList[j].ID
		})

		// Set data to Redis
		err = u.redisRepository.SetOnuIDCtx(ctx, redisKey, u.cfg.CacheCfg.EmptyOnuIDTTL, emptyOnuIDList)
		if err != nil {
			logger.WithRequestID(ctx).Error("redis_save_empty_onu_ids_failed", zap.Error(err), zap.String("redis_key", redisKey))
			return nil, err
		}

		logger.WithRequestID(ctx).Info("empty_onu_ids_saved_to_redis", zap.String("redis_key", redisKey))

		return emptyOnuIDList, nil
	})

	if err != nil {
		logger.WithRequestID(ctx).Error("get_empty_onu_ids_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
		return nil, err
	}

	return result.([]model.OnuID), nil // Return cast result
}

// noCacheCtxKey marks a context that must bypass cached reads (forced fresh).
type noCacheCtxKey struct{}

// WithNoCache returns a context that makes cached-read usecase paths skip Redis
// and read live from SNMP (and refresh the cache). Set by the HTTP handler when
// the request carries ?nocache=true — used by write-olt-zte's pre-write ONU
// existence/uniqueness checks so a delete/replace right after a provision sees
// the OLT's real state instead of a stale serial-list cache.
func WithNoCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, noCacheCtxKey{}, true)
}

func noCacheFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(noCacheCtxKey{}).(bool)
	return v
}

func (u *onuUsecase) GetOnuIDAndSerialNumber(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error) {
	// Set key for simple flight
	key := fmt.Sprintf("onu_id_and_serial_number:%d:%d", boardID, ponID)

	// Using simple flight to prevent duplicate requests for the same data
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config based on Board ID and PON ID
		oltConfig, err := u.getOltConfig(boardID, ponID)
		if err != nil {
			logger.WithRequestID(ctx).Error("olt_config_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Check Redis cache first — unless the caller forced a fresh read
		// (WithNoCache). A forced read still walks SNMP and re-saves the cache
		// below, so it doubles as a cache refresh. Used by write-olt-zte's
		// pre-write existence checks (?nocache=true), which must see the OLT's
		// real state, not a possibly-stale snapshot.
		redisKey := u.cacheKey(fmt.Sprintf("board_%d_pon_%d_serial_list", boardID, ponID))
		if !noCacheFromContext(ctx) {
			cached, cacheErr := u.redisRepository.GetONUSerialList(ctx, redisKey)
			if cacheErr == nil && cached != nil {
				logger.WithRequestID(ctx).Info("onu_serial_list_cache_hit", zap.String("redis_key", redisKey))
				return cached, nil
			}
		}

		// Perform SNMP Walk to get ONU ID
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		onuIDList := make([]model.OnuID, 0) // Initialize ID list

		logger.WithRequestID(ctx).Info("fetching_onu_ids_and_serial_numbers_snmp_walk", zap.Int("board_id", boardID), zap.Int("pon_id", ponID))

		// Perform SNMP BulkWalk to get ONU ID and Name
		err = u.snmpRepository.BulkWalk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			idOnuID := utils.ExtractIDOnuID(pdu.Name) // Extract ID
			onuIDList = append(onuIDList, model.OnuID{
				Board: boardID,
				PON:   ponID,
				ID:    idOnuID,
			})
			return nil
		})
		if err != nil {
			logger.WithRequestID(ctx).Error("snmp_walk_onu_ids_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Create a slice of ONU Serial Number
		var onuSerialNumberList []model.OnuSerialNumber

		// Loop through onuIDList to get ONU Serial Number
		for _, onuInfo := range onuIDList {
			// Get Data ONU Serial Number from SNMP Walk using the getSerialNumber method
			onuSerialNumber, err := u.getSerialNumber(oltConfig.OnuSerialNumberOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuSerialNumberList = append(onuSerialNumberList, model.OnuSerialNumber{
					Board:        boardID,
					PON:          ponID,
					ID:           onuInfo.ID,
					SerialNumber: onuSerialNumber, // Add serial number to list
				})
			}
		}

		// Sort ONU Serial Number list based on ONU ID ascending
		sort.Slice(onuSerialNumberList, func(i, j int) bool {
			return onuSerialNumberList[i].ID < onuSerialNumberList[j].ID
		})

		// Save to Redis cache using a context DETACHED from the request. The SNMP
		// walk already completed, so the cache write must not be aborted just
		// because the original caller went away (a canceled/timed-out delete
		// existence check, or a singleflight leader that gave up). Tying the save
		// to the request context produced spurious "context canceled" errors and
		// left the cache unpopulated.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := u.redisRepository.SaveONUSerialList(saveCtx, redisKey, u.cfg.CacheCfg.ONUInfoTTL, onuSerialNumberList); err != nil {
			logger.WithRequestID(ctx).Error("redis_save_onu_serial_list_failed", zap.Error(err), zap.String("redis_key", redisKey))
		}
		cancel()

		return onuSerialNumberList, nil
	})

	if err != nil {
		logger.WithRequestID(ctx).Error("get_onu_ids_and_serial_numbers_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
		return nil, err
	}

	return result.([]model.OnuSerialNumber), nil // Return cast result
}

func (u *onuUsecase) UpdateEmptyOnuID(ctx context.Context, boardID, ponID int) error {
	// Set key for simple flight
	key := fmt.Sprintf("update_empty_onu_id:%d:%d", boardID, ponID)

	// Using simple flight to prevent duplicate requests for the same data
	_, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config based on Board ID and PON ID
		oltConfig, err := u.getOltConfig(boardID, ponID)
		if err != nil {
			logger.WithRequestID(ctx).Error("olt_config_failed", zap.Error(err), zap.Int("board_id", boardID), zap.Int("pon_id", ponID))
			return nil, err
		}

		// Perform SNMP Walk to get ONU ID and ONU Name
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		emptyOnuIDList := make([]model.OnuID, 0) // Initialize an empty list

		logger.WithRequestID(ctx).Info("updating_empty_onu_ids_snmp_walk", zap.Int("board_id", boardID), zap.Int("pon_id", ponID))

		// Perform SNMP BulkWalk to get ONU ID and Name
		err = u.snmpRepository.BulkWalk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			idOnuID := utils.ExtractIDOnuID(pdu.Name) // Extract ID
			emptyOnuIDList = append(emptyOnuIDList, model.OnuID{
				Board: boardID,
				PON:   ponID,
				ID:    idOnuID,
			})
			return nil
		})
		if err != nil {
			return nil, apperrors.NewSNMPError("Walk", err) // Return SNMP error
		}

		// Create a map to store numbers to be deleted
		numbersToRemove := make(map[int]bool)
		for _, onuInfo := range emptyOnuIDList {
			numbersToRemove[onuInfo.ID] = true // Mark ID as existing
		}

		// Filter out ONU IDs that are not empty
		emptyOnuIDList = emptyOnuIDList[:0]
		for i := 1; i <= MaxOnuIDPerPon; i++ {
			if _, ok := numbersToRemove[i]; !ok { // If ID not marked, it is empty
				emptyOnuIDList = append(emptyOnuIDList, model.OnuID{
					Board: boardID,
					PON:   ponID,
					ID:    i,
				})
			}
		}

		// Sort ONU IDs by ID ascending
		sort.Slice(emptyOnuIDList, func(i, j int) bool {
			return emptyOnuIDList[i].ID < emptyOnuIDList[j].ID
		})

		// Set data to Redis using the SetOnuIDCtx method and helper function
		redisKey := u.cacheKey(GenerateRedisKey(RedisKeyTypeEmptyOnuID, boardID, ponID))
		err = u.redisRepository.SetOnuIDCtx(ctx, redisKey, u.cfg.CacheCfg.EmptyOnuIDTTL, emptyOnuIDList)
		if err != nil {
			logger.WithRequestID(ctx).Error("redis_update_empty_onu_ids_failed", zap.Error(err), zap.String("redis_key", redisKey))
			return nil, apperrors.NewRedisError("Set", err) // Return Redis error
		}

		logger.WithRequestID(ctx).Info("empty_onu_ids_updated_in_redis", zap.String("redis_key", redisKey))
		return nil, nil
	})

	return err // Return error if any
}

func (u *onuUsecase) GetByBoardIDAndPonIDWithPagination(
	ctx context.Context, boardID, ponID, pageIndex, pageSize int,
) ([]model.ONUInfoPerBoard, int) {
	// Get full ONU list (cached via Redis or fresh from SNMP)
	allOnus, err := u.GetByBoardIDAndPonID(ctx, boardID, ponID)
	if err != nil {
		return nil, 0
	}

	count := len(allOnus)

	// Calculate pagination bounds
	startIndex := (pageIndex - 1) * pageSize
	if startIndex >= count {
		return []model.ONUInfoPerBoard{}, count
	}

	endIndex := startIndex + pageSize
	if endIndex > count {
		endIndex = count
	}

	return allOnus[startIndex:endIndex], count
}

func (u *onuUsecase) getName(OnuIDNameOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuIDNameOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)         // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractName(result.Variables[0].Value), nil // Extract and return name
}

func (u *onuUsecase) getONUType(OnuTypeOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID2 + OnuTypeOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)       // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractName(result.Variables[0].Value), nil // Extract and return type
}

func (u *onuUsecase) getSerialNumber(OnuSerialNumberOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuSerialNumberOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)               // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractSerialNumber(result.Variables[0].Value), nil // Extract and return a serial number
}

func (u *onuUsecase) getTxPower(OnuTxPowerOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID2 + OnuTxPowerOID + "." + onuID + SNMPOIDSuffix // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)                          // Fetch from SNMP
	if err != nil {
		return "", err
	}
	power, _ := utils.ConvertAndMultiply(result.Variables[0].Value) // Convert power value
	return power, nil                                               // Return power
}

func (u *onuUsecase) getRxPower(OnuRxPowerOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuRxPowerOID + "." + onuID + SNMPOIDSuffix // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)                          // Fetch from SNMP
	if err != nil {
		return "", err
	}
	power, _ := utils.ConvertAndMultiply(result.Variables[0].Value) // Convert power value
	return power, nil                                               // Return power
}

func (u *onuUsecase) getStatus(OnuStatusOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuStatusOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)         // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractAndGetStatus(result.Variables[0].Value), nil // Extract and return status
}

func (u *onuUsecase) getIPAddress(OnuIPAddressOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID2 + OnuIPAddressOID + "." + onuID + SNMPOIDSuffix // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)                            // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractName(result.Variables[0].Value), nil // Extract and return IP
}

func (u *onuUsecase) getDescription(OnuDescriptionOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuDescriptionOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)              // Fetch from SNMP
	if err != nil {
		return "", err
	}
	return utils.ExtractName(result.Variables[0].Value), nil // Extract and return description
}

func (u *onuUsecase) getLastOnline(OnuLastOnlineOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuLastOnlineOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)             // Fetch from SNMP
	if err != nil {
		return "", err
	}

	value, ok := result.Variables[0].Value.([]byte) // Get value as bytes
	if !ok {
		return "", fmt.Errorf("unexpected SNMP value type for last online")
	}
	return utils.ConvertByteArrayToDateTime(value) // Convert to DateTime
}

func (u *onuUsecase) getLastOffline(OnuLastOfflineOID, onuID string) (string, error) {
	baseOID := u.cfg.OltCfg.BaseOID1                 // Get base OID
	oid := baseOID + OnuLastOfflineOID + "." + onuID // Construct full OID
	oids := []string{oid}                            // Create slice of OIDs

	result, err, _ := u.sg.Do(oid, func() (interface{}, error) {
		return u.snmpRepository.Get(oids) // Perform SNMP GET
	})
	if err != nil {
		logger.Error("snmp_get_last_offline_failed", zap.Error(err), zap.String("oid", oid))
		return "", apperrors.NewSNMPError("Get", err)
	}

	resultData := result.(*gosnmp.SnmpPacket) // Case result to SnmpPacket
	if len(resultData.Variables) > 0 {
		value, ok := resultData.Variables[0].Value.([]byte) // Get value
		if !ok {
			return "", fmt.Errorf("unexpected SNMP value type for last offline")
		}
		return utils.ConvertByteArrayToDateTime(value) // Convert to DateTime
	}

	logger.Error("snmp_get_last_offline_no_variables", zap.String("oid", oid))
	return "", apperrors.NewSNMPError("Get", fmt.Errorf("no variables in response")) // Return error
}

func (u *onuUsecase) getLastOfflineReason(OnuLastOfflineReasonOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuLastOfflineReasonOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)                    // Fetch from SNMP
	if err != nil {
		return "", err
	}

	return utils.ExtractLastOfflineReason(result.Variables[0].Value), nil // Extract and return reason
}

func (u *onuUsecase) getOnuGponOpticalDistance(OnuGponOpticalDistanceOID, onuID string) (string, error) {
	oid := u.cfg.OltCfg.BaseOID1 + OnuGponOpticalDistanceOID + "." + onuID // Construct OID
	result, err := u.getFromSNMPWithSingleflight(oid)                      // Fetch from SNMP
	if err != nil {
		return "", err
	}

	return utils.ExtractGponOpticalDistance(result.Variables[0].Value), nil // Extract and return distance
}

// oltLocation resolves the OLT's clock timezone (OLT_TIMEZONE, default
// Asia/Jakarta) to a *time.Location. The IANA tz database is embedded
// (time/tzdata in main), so this works in the distroless image; if the name is
// invalid it falls back to a fixed WIB (+7) zone so uptime never regresses to
// the old UTC bug.
func (u *onuUsecase) oltLocation() *time.Location {
	tz := u.cfg.OltCfg.Timezone
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	logger.Warn("olt_timezone_load_failed_fallback_wib", zap.String("timezone", tz))
	return time.FixedZone("WIB", int(TimezoneOffsetWIB/time.Second))
}

func (u *onuUsecase) getUptimeDuration(lastOnline string) (string, error) {
	// The OLT reports last_online in its own local wall-clock (e.g. WIB). Parse
	// it IN that timezone so it becomes the correct absolute instant, then diff
	// against the real "now" — dynamic for any OLT_TIMEZONE, and never the
	// ~7h-negative value the old parse-as-UTC produced.
	loc := u.oltLocation()
	lastOnlineTime, err := time.ParseInLocation(DateTimeFormat, lastOnline, loc)
	if err != nil {
		logger.Error("parse_last_online_failed", zap.Error(err), zap.String("last_online", lastOnline))
		return "", err
	}

	duration := time.Since(lastOnlineTime) // both are absolute instants now
	if duration < 0 {
		duration = 0 // Guard against clock skew producing a tiny negative uptime
	}
	return utils.ConvertDurationToString(duration), nil // Convert to string and return
}

// Last Down Duration
func (u *onuUsecase) getLastDownDuration(lastOffline, lastOnline string) (string, error) {
	// An ONU that has never gone offline reports an empty last-offline (and/or
	// last-online) timestamp. That is a normal state, not an error — return an
	// empty duration without logging, so logs aren't spammed for healthy ONUs.
	if strings.TrimSpace(lastOffline) == "" || strings.TrimSpace(lastOnline) == "" {
		return "", nil
	}

	lastOfflineTime, err := time.Parse(DateTimeFormat, lastOffline) // Parse last offline time
	if err != nil {
		logger.Error("parse_last_offline_failed", zap.Error(err), zap.String("last_offline", lastOffline))
		return "", err
	}

	lastOnlineTime, err := time.Parse(DateTimeFormat, lastOnline) // Parse last online time
	if err != nil {
		logger.Error("parse_last_online_failed", zap.Error(err), zap.String("last_online", lastOnline))
		return "", err
	}

	duration := lastOnlineTime.Sub(lastOfflineTime)     // Calculate difference
	return utils.ConvertDurationToString(duration), nil // Convert to string and return
}

func (u *onuUsecase) getFromSNMPWithSingleflight(oid string) (*gosnmp.SnmpPacket, error) {
	result, err, _ := u.sg.Do(oid, func() (interface{}, error) {
		return u.snmpRepository.Get([]string{oid}) // Get OID from SNMP
	})
	if err != nil {
		logger.Error("snmp_get_failed", zap.Error(err), zap.String("oid", oid))
		return nil, apperrors.NewSNMPError("Get", err)
	}

	packet := result.(*gosnmp.SnmpPacket) // Cast result
	if len(packet.Variables) == 0 {
		logger.Error("snmp_get_no_variables", zap.String("oid", oid))
		return nil, apperrors.NewSNMPError("Get", fmt.Errorf("no variables in response"))
	}

	return packet, nil // Return packet
}

// DeleteCache deletes the cached ONU information for a specific board and PON
func (u *onuUsecase) DeleteCache(ctx context.Context, boardID, ponID int) error {
	logger.WithRequestID(ctx).Info("deleting_cache_for_board_pon",
		zap.Int("board_id", boardID),
		zap.Int("pon_id", ponID),
	)

	// Validate board and pon IDs
	if _, err := u.getBoardConfig(boardID, ponID); err != nil {
		logger.WithRequestID(ctx).Error("invalid_board_pon_combination",
			zap.Error(err),
			zap.Int("board_id", boardID),
			zap.Int("pon_id", ponID),
		)
		return apperrors.NewValidationError("invalid board/pon combination",
			map[string]interface{}{"board_id": boardID, "pon_id": ponID})
	}

	// Delete the SAME namespaced key the read path writes (the read uses
	// u.cacheKey(...)). Without cacheKey this deleted the wrong key for a named
	// OLT (e.g. olt_c300_onu_info_...), so a multi-OLT cache never cleared.
	redisKey := u.cacheKey(GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID))

	// Delete from Redis
	err := u.redisRepository.Delete(ctx, redisKey)
	if err != nil {
		logger.WithRequestID(ctx).Error("redis_delete_cache_failed",
			zap.Error(err),
			zap.String("redis_key", redisKey),
		)
		return apperrors.NewRedisError("delete cache", err)
	}

	// Also delete serial list cache
	serialKey := u.cacheKey(fmt.Sprintf("board_%d_pon_%d_serial_list", boardID, ponID))
	if err := u.redisRepository.Delete(ctx, serialKey); err != nil {
		logger.WithRequestID(ctx).Debug("delete_serial_list_cache_failed", zap.Error(err), zap.String("key", serialKey))
	}

	logger.WithRequestID(ctx).Info("cache_deleted_successfully",
		zap.String("redis_key", redisKey),
		zap.Int("board_id", boardID),
		zap.Int("pon_id", ponID),
	)

	return nil
}

// InvalidateONUCache deletes the cached ONU detail and board/pon cache so
// the next fetch goes directly to SNMP for fresh data. Called by trap handler
// before status verification to avoid stale cache false negatives.
func (u *onuUsecase) InvalidateONUCache(ctx context.Context, boardID, ponID, onuID int) error {
	detailKey := u.cacheKey(GenerateONUDetailRedisKey(boardID, ponID, onuID))
	_ = u.redisRepository.Delete(ctx, detailKey)

	boardPonKey := u.cacheKey(GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID))
	_ = u.redisRepository.Delete(ctx, boardPonKey)

	// Also clear the separate serial-list cache (onu_id_sn) — it has its own key
	// and was previously left stale here, so a freshly provisioned/removed ONU
	// kept failing later existence checks. Keeps it coherent with the ONU list.
	serialKey := u.cacheKey(fmt.Sprintf("board_%d_pon_%d_serial_list", boardID, ponID))
	_ = u.redisRepository.Delete(ctx, serialKey)

	logger.Info("onu_cache_invalidated",
		zap.Int("board", boardID),
		zap.Int("pon", ponID),
		zap.Int("onu_id", onuID))

	return nil
}
