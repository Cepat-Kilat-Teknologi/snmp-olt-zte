package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/go-redis/redismock/v9"
)

func TestNewOnuRedisRepo(t *testing.T) {
	db, _ := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	if repo == nil {
		t.Error("Expected non-nil repository")
	}
}

func TestGetOnuIDCtx_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	expectedData := []model.OnuID{
		{Board: 1, PON: 1, ID: 1},
		{Board: 1, PON: 1, ID: 2},
	}

	dataBytes, _ := json.Marshal(expectedData)
	mock.ExpectGet(key).SetVal(string(dataBytes))

	result, err := repo.GetOnuIDCtx(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != len(expectedData) {
		t.Errorf("Expected %d items, got %d", len(expectedData), len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetOnuIDCtx_RedisError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).SetErr(errors.New("redis connection error"))

	result, err := repo.GetOnuIDCtx(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetOnuIDCtx_UnmarshalError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	// Return invalid JSON
	mock.ExpectGet(key).SetVal("invalid json")

	result, err := repo.GetOnuIDCtx(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSetOnuIDCtx_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.OnuID{
		{Board: 1, PON: 1, ID: 1},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetVal("OK")

	err := repo.SetOnuIDCtx(ctx, key, seconds, data)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSetOnuIDCtx_RedisError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.OnuID{
		{Board: 1, PON: 1, ID: 1},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetErr(errors.New("redis write error"))

	err := repo.SetOnuIDCtx(ctx, key, seconds, data)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestDeleteOnuIDCtx_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectDel(key).SetVal(1)

	err := repo.DeleteOnuIDCtx(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestDeleteOnuIDCtx_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectDel(key).SetErr(errors.New("redis delete error"))

	err := repo.DeleteOnuIDCtx(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveONUInfoList_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.ONUInfoPerBoard{
		{Board: 1, PON: 1, ID: 1, Name: "ONU1"},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetVal("OK")

	err := repo.SaveONUInfoList(ctx, key, seconds, data)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveONUInfoList_RedisError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.ONUInfoPerBoard{
		{Board: 1, PON: 1, ID: 1, Name: "ONU1"},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetErr(errors.New("redis error"))

	err := repo.SaveONUInfoList(ctx, key, seconds, data)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUInfoList_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	expectedData := []model.ONUInfoPerBoard{
		{Board: 1, PON: 1, ID: 1, Name: "ONU1"},
	}

	dataBytes, _ := json.Marshal(expectedData)
	mock.ExpectGet(key).SetVal(string(dataBytes))

	result, err := repo.GetONUInfoList(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != len(expectedData) {
		t.Errorf("Expected %d items, got %d", len(expectedData), len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUInfoList_CacheMiss(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).RedisNil()

	result, err := repo.GetONUInfoList(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUInfoList_UnmarshalError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).SetVal("invalid json")

	result, err := repo.GetONUInfoList(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetOnlyOnuIDCtx_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	expectedData := []model.OnuOnlyID{
		{ID: 1},
	}

	dataBytes, _ := json.Marshal(expectedData)
	mock.ExpectGet(key).SetVal(string(dataBytes))

	result, err := repo.GetOnlyOnuIDCtx(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != len(expectedData) {
		t.Errorf("Expected %d items, got %d", len(expectedData), len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetOnlyOnuIDCtx_CacheMiss(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).RedisNil()

	result, err := repo.GetOnlyOnuIDCtx(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetOnlyOnuIDCtx_UnmarshalError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).SetVal("invalid json")

	result, err := repo.GetOnlyOnuIDCtx(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveOnlyOnuIDCtx_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.OnuOnlyID{
		{ID: 1},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetVal("OK")

	err := repo.SaveOnlyOnuIDCtx(ctx, key, seconds, data)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveOnlyOnuIDCtx_RedisError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	data := []model.OnuOnlyID{
		{ID: 1},
	}

	dataBytes, _ := json.Marshal(data)
	mock.ExpectSet(key, dataBytes, time.Duration(seconds)*time.Second).SetErr(errors.New("redis error"))

	err := repo.SaveOnlyOnuIDCtx(ctx, key, seconds, data)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestDelete_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectDel(key).SetVal(1)

	err := repo.Delete(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestDelete_KeyNotFound(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	// Return 0 when key not found
	mock.ExpectDel(key).SetVal(0)

	err := repo.Delete(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestDelete_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectDel(key).SetErr(errors.New("redis delete error"))

	err := repo.Delete(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetTTL_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectTTL(key).SetVal(300 * time.Second)

	ttl, err := repo.GetTTL(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if ttl != 300*time.Second {
		t.Errorf("Expected TTL 300s, got %v", ttl)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetTTL_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectTTL(key).SetErr(errors.New("redis error"))

	_, err := repo.GetTTL(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveONUDetail_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	detail := model.ONUCustomerInfo{
		Board: 1, PON: 1, ID: 1, Name: "ONU1",
	}

	detailBytes, _ := json.Marshal(detail)
	mock.ExpectSet(key, detailBytes, time.Duration(seconds)*time.Second).SetVal("OK")

	err := repo.SaveONUDetail(ctx, key, seconds, detail)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestSaveONUDetail_RedisError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	seconds := 600
	detail := model.ONUCustomerInfo{
		Board: 1, PON: 1, ID: 1, Name: "ONU1",
	}

	detailBytes, _ := json.Marshal(detail)
	mock.ExpectSet(key, detailBytes, time.Duration(seconds)*time.Second).SetErr(errors.New("redis error"))

	err := repo.SaveONUDetail(ctx, key, seconds, detail)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUDetail_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"
	expectedData := model.ONUCustomerInfo{
		Board: 1, PON: 1, ID: 1, Name: "ONU1",
	}

	dataBytes, _ := json.Marshal(expectedData)
	mock.ExpectGet(key).SetVal(string(dataBytes))

	result, err := repo.GetONUDetail(ctx, key)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Name != expectedData.Name {
		t.Errorf("Expected name '%s', got '%s'", expectedData.Name, result.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUDetail_CacheMiss(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).RedisNil()

	result, err := repo.GetONUDetail(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}

func TestGetONUDetail_InvalidJSON(t *testing.T) {
	db, mock := redismock.NewClientMock()
	repo := NewOnuRedisRepo(db)

	ctx := context.Background()
	key := "test_key"

	mock.ExpectGet(key).SetVal("invalid json")

	result, err := repo.GetONUDetail(ctx, key)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unmet expectations: %v", err)
	}
}
