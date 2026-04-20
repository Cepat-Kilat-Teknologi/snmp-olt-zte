package repository

import (
	"context"
	"encoding/json"
	"time"

	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var jsonMarshal = json.Marshal

// OnuRedisRepositoryInterface is an interface that represents the auth's repository contract
// It defines the methods for interacting with Redis related to ONU data.
type OnuRedisRepositoryInterface interface {
	GetOnuIDCtx(ctx context.Context, key string) ([]model.OnuID, error)                                      // Get ONU IDs from Redis
	SetOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuID) error                     // Set ONU IDs to Redis with expiration
	DeleteOnuIDCtx(ctx context.Context, key string) error                                                    // Delete ONU IDs from Redis
	SaveONUInfoList(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error // Save the list of ONU info to Redis
	GetONUInfoList(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error)                         // Get the list of ONU info from Redis
	GetOnlyOnuIDCtx(ctx context.Context, key string) ([]model.OnuOnlyID, error)                              // Get only ONU IDs from Redis
	SaveOnlyOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuOnlyID) error            // Save only ONU IDs to Redis
	Delete(ctx context.Context, key string) error                                                            // Delete any key from Redis
	GetTTL(ctx context.Context, key string) (time.Duration, error)                                           // Get TTL for a key
	SaveONUDetail(ctx context.Context, key string, seconds int, detail model.ONUCustomerInfo) error          // Save ONU detail to Redis
	GetONUDetail(ctx context.Context, key string) (*model.ONUCustomerInfo, error)                            // Get ONU detail from Redis
	SaveONUSerialList(ctx context.Context, key string, seconds int, list []model.OnuSerialNumber) error      // Save ONU serial number list to Redis
	GetONUSerialList(ctx context.Context, key string) ([]model.OnuSerialNumber, error)                       // Get ONU serial number list from Redis
}

// Auth redis repository
// onuRedisRepo implements OnuRedisRepositoryInterface
type onuRedisRepo struct {
	redisClient *redis.Client // Redis client instance
}

// NewOnuRedisRepo will create an object that represents the auth repository
func NewOnuRedisRepo(redisClient *redis.Client) OnuRedisRepositoryInterface { // Constructor for OnuRedisRepository
	return &onuRedisRepo{redisClient} // Return a new instance with an injected client
}

// GetOnuIDCtx is a method to get onu id from redis
func (r *onuRedisRepo) GetOnuIDCtx(ctx context.Context, key string) ([]model.OnuID, error) {
	onuBytes, err := r.redisClient.Get(ctx, key).Bytes() // Get value as bytes from Redis using a key

	// Check for error
	if err != nil {
		// Cache miss is normal behavior, not an error - log as debug only
		logger.WithRequestID(ctx).Debug("cache_miss_key_not_found", zap.String("key", key))
		return nil, apperrors.NewRedisError("Get", err) // Return wrapped Redis error
	}

	var onuID []model.OnuID                                  // Variable to hold the result
	if err := json.Unmarshal(onuBytes, &onuID); err != nil { // Unmarshal JSON bytes into onuID slice
		logger.WithRequestID(ctx).Error("failed_to_unmarshal_onu_id", zap.Error(err))
		return nil, apperrors.NewInternalError("failed to unmarshal onu id", err) // Return wrapped internal error
	}

	return onuID, nil // Return the result and nil error
}

// SetOnuIDCtx is a method to set onu id to redis
func (r *onuRedisRepo) SetOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
	// Marshal onuID slice to JSON bytes
	// Note: json.Marshal cannot fail for model.OnuID (only contains int fields)
	onuBytes, _ := json.Marshal(onuID)

	// Set the key in Redis with the marshaled bytes and expiration time
	if err := r.redisClient.Set(ctx, key, onuBytes, time.Second*time.Duration(seconds)).Err(); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_set_onu_id_to_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Set", err) // Return wrapped Redis error
	}

	return nil // Return nil on success
}

// DeleteOnuIDCtx is a method to delete onu id from redis
func (r *onuRedisRepo) DeleteOnuIDCtx(ctx context.Context, key string) error {
	if err := r.redisClient.Del(ctx, key).Err(); err != nil { // Delete key from Redis
		logger.WithRequestID(ctx).Error("failed_to_delete_onu_id_from_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Del", err) // Return wrapped Redis error
	}

	return nil // Return nil on success
}

// SaveONUInfoList is a method to save one info list to redis
func (r *onuRedisRepo) SaveONUInfoList(
	ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard,
) error {
	// Marshal list to JSON bytes
	// Note: json.Marshal cannot fail for model.ONUInfoPerBoard (only contains int/string fields)
	onuBytes, _ := json.Marshal(onuInfoList)

	// Set key in Redis with expiration
	if err := r.redisClient.Set(ctx, key, onuBytes, time.Second*time.Duration(seconds)).Err(); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_set_onu_info_list_to_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Set", err) // Return wrapped Redis error
	}

	return nil // Return nil on success
}

// GetONUInfoList is a method to get one info list from redis
func (r *onuRedisRepo) GetONUInfoList(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
	onuBytes, err := r.redisClient.Get(ctx, key).Bytes() // Get value as bytes from Redis
	if err != nil {                                      // Check for error
		// Cache miss is normal behavior, not an error - log as debug only
		logger.WithRequestID(ctx).Debug("cache_miss_key_not_found", zap.String("key", key))
		return nil, apperrors.NewRedisError("Get", err) // Return wrapped Redis error
	}

	var onuInfoList []model.ONUInfoPerBoard                        // Variable to hold a result
	if err := json.Unmarshal(onuBytes, &onuInfoList); err != nil { // Unmarshal JSON to struct
		logger.WithRequestID(ctx).Error("failed_to_unmarshal_onu_info_list", zap.Error(err))
		return nil, apperrors.NewInternalError("failed to unmarshal onu info list", err) // Return wrapped internal error
	}

	return onuInfoList, nil // Return result
}

// GetOnlyOnuIDCtx is a method to get only onu id from redis
func (r *onuRedisRepo) GetOnlyOnuIDCtx(ctx context.Context, key string) ([]model.OnuOnlyID, error) {
	onuBytes, err := r.redisClient.Get(ctx, key).Bytes() // Get value as bytes from Redis
	if err != nil {                                      // Check for error
		// Cache miss is normal behavior, not an error - log as debug only
		logger.WithRequestID(ctx).Debug("cache_miss_key_not_found", zap.String("key", key))
		return nil, apperrors.NewRedisError("Get", err) // Return wrapped Redis error
	}

	var onuID []model.OnuOnlyID                              // Variable to hold a result
	if err := json.Unmarshal(onuBytes, &onuID); err != nil { // Unmarshal JSON
		logger.WithRequestID(ctx).Error("failed_to_unmarshal_onu_id", zap.Error(err))
		return nil, apperrors.NewInternalError("failed to unmarshal onu id", err) // Return wrapped internal error
	}

	return onuID, nil // Return result
}

// SaveOnlyOnuIDCtx is a method to save only onu id to redis
func (r *onuRedisRepo) SaveOnlyOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuOnlyID) error {
	// Marshal struct to JSON
	// Note: json.Marshal cannot fail for model.OnuOnlyID (only contains int field)
	onuBytes, _ := json.Marshal(onuID)

	// Set key in Redis with expiration
	if err := r.redisClient.Set(ctx, key, onuBytes, time.Second*time.Duration(seconds)).Err(); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_set_onu_id_to_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Set", err) // Return wrapped Redis error
	}

	return nil // Return nil
}

// Delete is a method to delete any key from redis
func (r *onuRedisRepo) Delete(ctx context.Context, key string) error {
	// Delete key from Redis
	result, err := r.redisClient.Del(ctx, key).Result()
	if err != nil {
		logger.WithRequestID(ctx).Error("failed_to_delete_key_from_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Delete", err)
	}

	// Log result
	if result == 0 {
		logger.WithRequestID(ctx).Warn("key_not_found_in_redis", zap.String("key", key))
	} else {
		logger.WithRequestID(ctx).Info("successfully_deleted_key_from_redis", zap.String("key", key), zap.Int64("deleted_count", result))
	}

	return nil
}

// GetTTL returns the remaining TTL for a key in Redis
func (r *onuRedisRepo) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return r.redisClient.TTL(ctx, key).Result()
}

// SaveONUDetail saves ONU detail information to Redis with expiration
func (r *onuRedisRepo) SaveONUDetail(ctx context.Context, key string, seconds int, detail model.ONUCustomerInfo) error {
	detailBytes, err := jsonMarshal(detail)
	if err != nil {
		logger.WithRequestID(ctx).Error("failed_to_marshal_onu_detail", zap.Error(err))
		return apperrors.NewInternalError("failed to marshal ONU detail", err)
	}

	// Set key in Redis with expiration
	if err := r.redisClient.Set(ctx, key, detailBytes, time.Second*time.Duration(seconds)).Err(); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_set_onu_detail_to_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Set", err)
	}

	return nil
}

// GetONUDetail retrieves ONU detail information from Redis
func (r *onuRedisRepo) GetONUDetail(ctx context.Context, key string) (*model.ONUCustomerInfo, error) {
	detailBytes, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		logger.WithRequestID(ctx).Debug("cache_miss_onu_detail_key_not_found", zap.String("key", key))
		return nil, apperrors.NewRedisError("Get", err)
	}

	var detail model.ONUCustomerInfo
	if err := json.Unmarshal(detailBytes, &detail); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_unmarshal_onu_detail", zap.Error(err))
		return nil, apperrors.NewInternalError("failed to unmarshal ONU detail", err)
	}

	return &detail, nil
}

// SaveONUSerialList saves ONU serial number list to Redis
func (r *onuRedisRepo) SaveONUSerialList(ctx context.Context, key string, seconds int, list []model.OnuSerialNumber) error {
	dataBytes, err := jsonMarshal(list)
	if err != nil {
		logger.WithRequestID(ctx).Error("failed_to_marshal_onu_serial_list", zap.Error(err))
		return apperrors.NewInternalError("failed to marshal ONU serial list", err)
	}
	if err := r.redisClient.Set(ctx, key, dataBytes, time.Second*time.Duration(seconds)).Err(); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_set_onu_serial_list_to_redis", zap.Error(err), zap.String("key", key))
		return apperrors.NewRedisError("Set", err)
	}
	return nil
}

// GetONUSerialList retrieves ONU serial number list from Redis
func (r *onuRedisRepo) GetONUSerialList(ctx context.Context, key string) ([]model.OnuSerialNumber, error) {
	dataBytes, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		logger.WithRequestID(ctx).Debug("cache_miss_onu_serial_list_not_found", zap.String("key", key))
		return nil, apperrors.NewRedisError("Get", err)
	}
	var list []model.OnuSerialNumber
	if err := json.Unmarshal(dataBytes, &list); err != nil {
		logger.WithRequestID(ctx).Error("failed_to_unmarshal_onu_serial_list", zap.Error(err))
		return nil, apperrors.NewInternalError("failed to unmarshal ONU serial list", err)
	}
	return list, nil
}
