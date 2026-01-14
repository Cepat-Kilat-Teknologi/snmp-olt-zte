package usecase

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/gosnmp/gosnmp"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// Constants for ONU configuration limits and cache TTL values
const (
	// MaxOnuIDPerPon is the maximum number of ONU IDs allowed per PON port
	MaxOnuIDPerPon = 128

	// RedisONUInfoTTL is the TTL in seconds for ONU info cache (10 minutes)
	RedisONUInfoTTL = 600

	// RedisEmptyOnuIDTTL is the TTL in seconds for empty ONU ID cache (5 minutes)
	RedisEmptyOnuIDTTL = 300

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

// OnuUseCaseInterface is an interface that represents the auth's usecase contract
type OnuUseCaseInterface interface {
	GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)        // Get ONU info by board and PON
	GetByBoardIDPonIDAndOnuID(boardID, ponID, onuID int) (model.ONUCustomerInfo, error)                   // Get specific ONU info
	GetEmptyOnuID(ctx context.Context, boardID, ponID int) ([]model.OnuID, error)                         // Get empty ONU IDs
	GetOnuIDAndSerialNumber(boardID, ponID int) ([]model.OnuSerialNumber, error)                          // Get ONU IDs and serial numbers
	UpdateEmptyOnuID(ctx context.Context, boardID, ponID int) error                                       // Update empty ONU IDs cache
	GetByBoardIDAndPonIDWithPagination(boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) // Get paginated ONU info
	DeleteCache(ctx context.Context, boardID, ponID int) error                                            // Delete cache for specific board/pon
}

// onuUsecase represent the auth's usecase
type onuUsecase struct {
	snmpRepository  repository.SnmpRepositoryInterface     // SNMP repository dependency
	redisRepository repository.OnuRedisRepositoryInterface // Redis repository dependency
	cfg             *config.Config                         // Configuration dependency
	sg              singleflight.Group                     // Singleflight group for request coalescing
}

// NewOnuUsecase will create an object that represents the auth usecase
func NewOnuUsecase(
	snmpRepository repository.SnmpRepositoryInterface, redisRepository repository.OnuRedisRepositoryInterface,
	cfg *config.Config,
) OnuUseCaseInterface {
	return &onuUsecase{
		snmpRepository:  snmpRepository,       // Inject SNMP repository
		redisRepository: redisRepository,      // Inject Redis repository
		cfg:             cfg,                  // Inject configuration
		sg:              singleflight.Group{}, // Initialize a singleflight group
	}
}

// getOltInfo is a function to get OLT information
func (u *onuUsecase) getOltConfig(boardID, ponID int) (*model.OltConfig, error) {
	cfg, err := u.getBoardConfig(boardID, ponID) // Retrieve board configuration
	if err != nil {
		log.Error().Msg(err.Error()) // Log error
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
	log.Info().
		Int("board_id", boardID).
		Int("pon_id", ponID).
		Msg("Getting all ONU information")

	key := fmt.Sprintf("onuinfo-b%d-p%d", boardID, ponID) // Create unique key for singleflight

	// Using simple flight to prevent duplicate SNMP requests
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config
		oltConfig, err := u.getOltConfig(boardID, ponID) // Get OLT config based on Board ID and PON ID
		if err != nil {
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get OLT Config")
			return nil, err
		}

		// Redis key using helper function
		redisKey := GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID)

		// Check if data is already cached in Redis
		cachedOnuData, err := u.redisRepository.GetONUInfoList(ctx, redisKey) // Get ONU Information from Redis
		if err == nil && cachedOnuData != nil {
			log.Info().Str("redis_key", redisKey).Msg("Retrieved ONU information from Redis cache")
			return cachedOnuData, nil
		}

		// SNMP Walk to get Information from OLT Board and PON
		log.Info().Int("board_id", boardID).Int("pon_id", ponID).Msg("Fetching ONU information via SNMP Walk")
		// Create a map to store SNMP Walk results
		snmpDataMap := make(map[string]gosnmp.SnmpPDU)
		// Perform SNMP Walk to get ONU ID and Name using snmpRepository Walk method with timeout context parameter
		err = u.snmpRepository.Walk(oltConfig.BaseOID+oltConfig.OnuIDNameOID, func(pdu gosnmp.SnmpPDU) error {
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

			// Sequential SNMP Gets (gosnmp is not thread-safe, parallel caused worse performance)
			// Get Data ONU Type from SNMP Walk using the getONUType method
			if onuType, err := u.getONUType(oltConfig.OnuTypeOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.OnuType = onuType
			}
			// Get Data ONU Serial Number from SNMP Walk using the getSerialNumber method
			if sn, err := u.getSerialNumber(oltConfig.OnuSerialNumberOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.SerialNumber = sn
			}
			// Get Data ONU RX Power from SNMP Walk using the getRxPower method
			if rx, err := u.getRxPower(oltConfig.OnuRxPowerOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.RXPower = rx
			}
			// Get Data ONU TX Power from SNMP Walk using getTxPower method
			if status, err := u.getStatus(oltConfig.OnuStatusOID, strconv.Itoa(onuInfo.ID)); err == nil {
				onuInfo.Status = status
			}

			// Add info to the list
			onuInformationList = append(onuInformationList, onuInfo)
		}

		// Sort the ONU information list by ID
		sort.Slice(onuInformationList, func(i, j int) bool {
			return onuInformationList[i].ID < onuInformationList[j].ID
		})

		// Save the ONU information list to Redis with a 10-minute expiration time
		// Balanced: 600s (10min) - fresh enough while maintaining a good cache hit rate
		err = u.redisRepository.SaveONUInfoList(ctx, redisKey, RedisONUInfoTTL, onuInformationList)
		if err != nil {
			log.Error().Err(err).Str("redis_key", redisKey).Msg("Failed to save ONU information to Redis")
		} else {
			log.Info().Str("redis_key", redisKey).Msg("Saved ONU information to Redis")
		}

		// Return the ONU information list
		return onuInformationList, nil
	})

	if err != nil {
		log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get ONU information")
		return nil, err
	}

	return result.([]model.ONUInfoPerBoard), nil // Return the result from the cache or SNMP Walk
}

func (u *onuUsecase) GetByBoardIDPonIDAndOnuID(boardID, ponID, onuID int) (
	model.ONUCustomerInfo, error,
) {
	// Set key for simple flight
	key := fmt.Sprintf("onu:%d:%d:%d", boardID, ponID, onuID)

	// Using simple flight to prevent duplicate SNMP requests
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		oltConfig, err := u.getOltConfig(boardID, ponID) // Get OLT config based on Board ID and PON ID
		if err != nil {
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get OLT Config")
			return model.ONUCustomerInfo{}, err
		}

		var onuInformationList model.ONUCustomerInfo   // Create a variable to store ONU information
		snmpDataMap := make(map[string]gosnmp.SnmpPDU) // Create a map to store SNMP Walk results

		log.Info().
			Int("board_id", boardID).
			Int("pon_id", ponID).
			Int("onu_id", onuID).
			Msg("Fetching detailed ONU information via SNMP Walk")

		// Get ONU ID and Name using snmpRepository Walk method with timeout context parameter
		err = u.snmpRepository.Walk(oltConfig.BaseOID+oltConfig.OnuIDNameOID+"."+strconv.Itoa(onuID),
			func(pdu gosnmp.SnmpPDU) error {
				snmpDataMap[utils.ExtractONUID(pdu.Name)] = pdu // Extract ID and store PDU
				return nil
			})
		if err != nil {
			log.Error().Err(err).Str("oid", oltConfig.BaseOID+oltConfig.OnuIDNameOID).Msg("Failed to walk OID")
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
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get OLT Config for empty ONU ID")
			return nil, err
		}

		// Redis Key using helper function
		redisKey := GenerateRedisKey(RedisKeyTypeEmptyOnuID, boardID, ponID)

		// Try to get data from Redis using the GetOnuIDCtx method with context and Redis key as a parameter
		cachedOnuData, err := u.redisRepository.GetOnuIDCtx(ctx, redisKey)
		if err == nil && cachedOnuData != nil {
			log.Info().Str("redis_key", redisKey).Msg("Retrieved empty ONU IDs from Redis cache")
			// If data exists in Redis, return data from Redis
			return cachedOnuData, nil
		}

		// Perform SNMP Walk to get ONU ID and ONU Name
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		emptyOnuIDList := make([]model.OnuID, 0) // Initialize an empty list

		log.Info().Int("board_id", boardID).Int("pon_id", ponID).Msg("Fetching empty ONU IDs via SNMP Walk")

		// Perform SNMP Walk to get ONU ID and Name
		err = u.snmpRepository.Walk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			idOnuID := utils.ExtractIDOnuID(pdu.Name) // Extract ID
			emptyOnuIDList = append(emptyOnuIDList, model.OnuID{
				Board: boardID,
				PON:   ponID,
				ID:    idOnuID,
			})
			return nil
		})
		if err != nil {
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to perform SNMP Walk for empty ONU IDs")
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
		err = u.redisRepository.SetOnuIDCtx(ctx, redisKey, RedisEmptyOnuIDTTL, emptyOnuIDList)
		if err != nil {
			log.Error().Err(err).Str("redis_key", redisKey).Msg("Failed to save empty ONU IDs to Redis")
			return nil, err
		}

		log.Info().Str("redis_key", redisKey).Msg("Saved empty ONU IDs to Redis")

		return emptyOnuIDList, nil
	})

	if err != nil {
		log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get empty ONU IDs")
		return nil, err
	}

	return result.([]model.OnuID), nil // Return cast result
}

func (u *onuUsecase) GetOnuIDAndSerialNumber(boardID, ponID int) ([]model.OnuSerialNumber, error) {
	// Set key for simple flight
	key := fmt.Sprintf("onu_id_and_serial_number:%d:%d", boardID, ponID)

	// Using simple flight to prevent duplicate requests for the same data
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config based on Board ID and PON ID
		oltConfig, err := u.getOltConfig(boardID, ponID)
		if err != nil {
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get OLT Config")
			return nil, err
		}

		// Perform SNMP Walk to get ONU ID
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		onuIDList := make([]model.OnuID, 0) // Initialize ID list

		log.Info().Int("board_id", boardID).Int("pon_id", ponID).Msg("Fetching ONU IDs and serial numbers via SNMP Walk")

		// Perform SNMP BulkWalk to get ONU ID and Name
		err = u.snmpRepository.Walk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			idOnuID := utils.ExtractIDOnuID(pdu.Name) // Extract ID
			onuIDList = append(onuIDList, model.OnuID{
				Board: boardID,
				PON:   ponID,
				ID:    idOnuID,
			})
			return nil
		})
		if err != nil {
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to perform SNMP Walk for ONU IDs")
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

		return onuSerialNumberList, nil
	})

	if err != nil {
		log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get ONU IDs and serial numbers")
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
			log.Error().Err(err).Int("board_id", boardID).Int("pon_id", ponID).Msg("Failed to get OLT Config")
			return nil, err
		}

		// Perform SNMP Walk to get ONU ID and ONU Name
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID
		emptyOnuIDList := make([]model.OnuID, 0) // Initialize an empty list

		log.Info().Int("board_id", boardID).Int("pon_id", ponID).Msg("Updating empty ONU IDs via SNMP Walk")

		// Perform SNMP BulkWalk to get ONU ID and Name
		err = u.snmpRepository.Walk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
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
		redisKey := GenerateRedisKey(RedisKeyTypeEmptyOnuID, boardID, ponID)
		err = u.redisRepository.SetOnuIDCtx(ctx, redisKey, RedisEmptyOnuIDTTL, emptyOnuIDList)
		if err != nil {
			log.Error().Err(err).Str("redis_key", redisKey).Msg("Failed to update empty ONU IDs in Redis")
			return nil, apperrors.NewRedisError("Set", err) // Return Redis error
		}

		log.Info().Str("redis_key", redisKey).Msg("Updated empty ONU IDs in Redis")
		return nil, nil
	})

	return err // Return error if any
}

func (u *onuUsecase) GetByBoardIDAndPonIDWithPagination(
	boardID, ponID, pageIndex, pageSize int,
) ([]model.ONUInfoPerBoard, int) {

	// Create a unique key for this request based on the parameters
	key := fmt.Sprintf("get_onu_info:%d:%d:%d:%d", boardID, ponID, pageIndex, pageSize)

	// Using simple flight to prevent duplicate requests for the same data
	result, err, _ := u.sg.Do(key, func() (interface{}, error) {
		// Get OLT config based on Board ID and PON ID
		oltConfig, err := u.getOltConfig(boardID, ponID)
		if err != nil {
			return nil, err // Return error if config fetch fails
		}

		// SNMP OID variable
		snmpOID := oltConfig.BaseOID + oltConfig.OnuIDNameOID

		var onlyOnuIDList []model.OnuOnlyID // List to store only IDs
		var count int                       // Total count

		// Get ONU IDs from SNMP Walk
		err = u.snmpRepository.Walk(snmpOID, func(pdu gosnmp.SnmpPDU) error {
			onlyOnuIDList = append(onlyOnuIDList, model.OnuOnlyID{
				ID: utils.ExtractIDOnuID(pdu.Name), // Extract ID
			})
			return nil
		})
		if err != nil {
			return nil, err // Return error if walk fails
		}

		// Calculate total count
		count = len(onlyOnuIDList)

		// Calculate the index of the first item to be retrieved
		startIndex := (pageIndex - 1) * pageSize

		// If startIndex is out of range, return empty result
		if startIndex >= count {
			return model.PaginationResult{
				OnuInformationList: []model.ONUInfoPerBoard{},
				Count:              count,
			}, nil
		}

		// Calculate the index of the last item to be retrieved
		endIndex := startIndex + pageSize

		// If the index of the last item to be retrieved is greater than the number of items, set it to the number of items
		if endIndex > len(onlyOnuIDList) {
			endIndex = len(onlyOnuIDList)
		}

		// Slice the data for pagination
		onlyOnuIDList = onlyOnuIDList[startIndex:endIndex]

		var onuInformationList []model.ONUInfoPerBoard // List for fully populated info

		// Loop through onlyOnuIDList to get ONU information based on ONU ID
		for _, onuID := range onlyOnuIDList {
			onuInfo := model.ONUInfoPerBoard{
				Board: boardID,  // Set Board ID to ONUInfo struct Board field
				PON:   ponID,    // Set PON ID to ONUInfo struct PON field
				ID:    onuID.ID, // Set ONU ID to ONUInfo struct ID field
			}

			// Get Name based on ONU ID and ONU Name OID and store it to ONU onuInfo struct
			onuName, err := u.getName(oltConfig.OnuIDNameOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuInfo.Name = onuName // Set ONU Name to ONU onuInfo struct Name field
			}

			// Get ONU Type based on ONU ID and ONU Type OID and store it to ONU onuInfo struct
			onuType, err := u.getONUType(oltConfig.OnuTypeOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuInfo.OnuType = onuType // Set ONU Type to ONU onuInfo struct OnuType field
			}

			// Get ONU Serial Number based on ONU ID and ONU Serial Number OID and store it to ONU onuInfo struct
			onuSerialNumber, err := u.getSerialNumber(oltConfig.OnuSerialNumberOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuInfo.SerialNumber = onuSerialNumber // Set ONU Serial Number to ONU onuInfo struct SerialNumber field
			}

			// Get ONU RX Power based on ONU ID and ONU RX Power OID and store it to ONU onuInfo struct
			onuRXPower, err := u.getRxPower(oltConfig.OnuRxPowerOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuInfo.RXPower = onuRXPower // Set ONU RX Power to ONU onuInfo struct RXPower field
			}

			// Get ONU Status based on ONU ID and ONU Status OID and store it to ONU onuInfo struct
			onuStatus, err := u.getStatus(oltConfig.OnuStatusOID, strconv.Itoa(onuInfo.ID))
			if err == nil {
				onuInfo.Status = onuStatus // Set ONU Status to ONU onuInfo struct Status field
			}

			// Append ONU information to the onuInformationList
			onuInformationList = append(onuInformationList, onuInfo)
		}

		// Sort ONU information list based on ONU ID ascending
		sort.Slice(onuInformationList, func(i, j int) bool {
			return onuInformationList[i].ID < onuInformationList[j].ID
		})

		// Return both the list and the count inside a struct
		return model.PaginationResult{
			OnuInformationList: onuInformationList,
			Count:              count,
		}, nil
	})

	// Handle error if any occurred during simple flight processing
	if err != nil {
		return nil, 0
	}

	// Extract the result from the simple flight result and return it
	paginationResult := result.(model.PaginationResult)
	return paginationResult.OnuInformationList, paginationResult.Count

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

	value := result.Variables[0].Value.([]byte)    // Get value as bytes
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
		log.Error().Err(err).Str("oid", oid).Msg("Failed to perform SNMP Get for last offline")
		return "", apperrors.NewSNMPError("Get", err)
	}

	resultData := result.(*gosnmp.SnmpPacket) // Case result to SnmpPacket
	if len(resultData.Variables) > 0 {
		value := resultData.Variables[0].Value.([]byte) // Get value
		return utils.ConvertByteArrayToDateTime(value)  // Convert to DateTime
	}

	log.Error().Str("oid", oid).Msg("Failed to get ONU Last Offline: No variables in response")
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

func (u *onuUsecase) getUptimeDuration(lastOnline string) (string, error) {
	currentTime := time.Now() // Get current time

	lastOnlineTime, err := time.Parse(DateTimeFormat, lastOnline) // Parse last online string
	if err != nil {
		log.Error().Err(err).Str("last_online", lastOnline).Msg("Failed to parse last online time")
		return "", err
	}

	duration := currentTime.Sub(lastOnlineTime) + TimezoneOffsetWIB // Calculate duration (adjusting for WIB timezone)
	return utils.ConvertDurationToString(duration), nil             // Convert to string and return
}

// Last Down Duration
func (u *onuUsecase) getLastDownDuration(lastOffline, lastOnline string) (string, error) {
	lastOfflineTime, err := time.Parse(DateTimeFormat, lastOffline) // Parse last offline time
	if err != nil {
		log.Error().Err(err).Str("last_offline", lastOffline).Msg("Failed to parse last offline time")
		return "", err
	}

	lastOnlineTime, err := time.Parse(DateTimeFormat, lastOnline) // Parse last online time
	if err != nil {
		log.Error().Err(err).Str("last_online", lastOnline).Msg("Failed to parse last online time")
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
		log.Error().Err(err).Str("oid", oid).Msg("Failed to perform SNMP Get")
		return nil, apperrors.NewSNMPError("Get", err)
	}

	packet := result.(*gosnmp.SnmpPacket) // Cast result
	if len(packet.Variables) == 0 {
		log.Error().Str("oid", oid).Msg("No variables returned for OID")
		return nil, apperrors.NewSNMPError("Get", fmt.Errorf("no variables in response"))
	}

	return packet, nil // Return packet
}

// DeleteCache deletes the cached ONU information for a specific board and PON
func (u *onuUsecase) DeleteCache(ctx context.Context, boardID, ponID int) error {
	log.Info().
		Int("board_id", boardID).
		Int("pon_id", ponID).
		Msg("Deleting cache for board/pon")

	// Validate board and pon IDs
	if _, err := u.getBoardConfig(boardID, ponID); err != nil {
		log.Error().Err(err).
			Int("board_id", boardID).
			Int("pon_id", ponID).
			Msg("Invalid board/pon combination")
		return apperrors.NewValidationError("invalid board/pon combination",
			map[string]interface{}{"board_id": boardID, "pon_id": ponID})
	}

	// Delete cache using the same key pattern as in GetByBoardIDAndPonID
	redisKey := GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID)

	// Delete from Redis
	err := u.redisRepository.Delete(ctx, redisKey)
	if err != nil {
		log.Error().Err(err).
			Str("redis_key", redisKey).
			Msg("Failed to delete cache from Redis")
		return apperrors.NewRedisError("delete cache", err)
	}

	log.Info().
		Str("redis_key", redisKey).
		Int("board_id", boardID).
		Int("pon_id", ponID).
		Msg("Successfully deleted cache")

	return nil
}
