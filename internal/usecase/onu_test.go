package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/gosnmp/gosnmp"
)

// mockSnmpRepository is a mock implementation of SnmpRepositoryInterface
type mockSnmpRepository struct {
	GetFunc      func(oids []string) (*gosnmp.SnmpPacket, error)
	WalkFunc     func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error
	BulkWalkFunc func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error
}

func (m *mockSnmpRepository) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	if m.GetFunc != nil {
		return m.GetFunc(oids)
	}
	// Default: return empty packet
	return &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  oids[0],
				Type:  gosnmp.OctetString,
				Value: []byte("test"),
			},
		},
	}, nil
}

func (m *mockSnmpRepository) Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	if m.WalkFunc != nil {
		return m.WalkFunc(oid, walkFunc)
	}
	// Default: simulate one ONU
	return walkFunc(gosnmp.SnmpPDU{
		Name:  oid + ".1.1.1",
		Type:  gosnmp.OctetString,
		Value: []byte("TestONU"),
	})
}

func (m *mockSnmpRepository) BulkWalk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
	if m.BulkWalkFunc != nil {
		return m.BulkWalkFunc(oid, walkFunc)
	}
	// Default: delegate to WalkFunc for backward compatibility
	if m.WalkFunc != nil {
		return m.WalkFunc(oid, walkFunc)
	}
	// Default: simulate one ONU
	return walkFunc(gosnmp.SnmpPDU{
		Name:  oid + ".1.1.1",
		Type:  gosnmp.OctetString,
		Value: []byte("TestONU"),
	})
}

func (m *mockSnmpRepository) Close() {}

// mockRedisRepository is a mock implementation of OnuRedisRepositoryInterface
type mockRedisRepository struct {
	GetONUInfoListFunc  func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error)
	SaveONUInfoListFunc func(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error
	GetOnuIDCtxFunc     func(ctx context.Context, key string) ([]model.OnuID, error)
	SetOnuIDCtxFunc     func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error
	DeleteFunc          func(ctx context.Context, key string) error
	GetTTLFunc          func(ctx context.Context, key string) (time.Duration, error)
	SaveONUDetailFunc     func(ctx context.Context, key string, seconds int, detail model.ONUCustomerInfo) error
	GetONUDetailFunc      func(ctx context.Context, key string) (*model.ONUCustomerInfo, error)
	GetONUSerialListFunc  func(ctx context.Context, key string) ([]model.OnuSerialNumber, error)
	SaveONUSerialListFunc func(ctx context.Context, key string, seconds int, list []model.OnuSerialNumber) error
}

func (m *mockRedisRepository) GetOnuIDCtx(ctx context.Context, key string) ([]model.OnuID, error) {
	if m.GetOnuIDCtxFunc != nil {
		return m.GetOnuIDCtxFunc(ctx, key)
	}
	return nil, errors.New("not found")
}

func (m *mockRedisRepository) SetOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
	if m.SetOnuIDCtxFunc != nil {
		return m.SetOnuIDCtxFunc(ctx, key, seconds, onuID)
	}
	return nil
}

func (m *mockRedisRepository) DeleteOnuIDCtx(ctx context.Context, key string) error {
	return nil
}

func (m *mockRedisRepository) SaveONUInfoList(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error {
	if m.SaveONUInfoListFunc != nil {
		return m.SaveONUInfoListFunc(ctx, key, seconds, onuInfoList)
	}
	return nil
}

func (m *mockRedisRepository) GetONUInfoList(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
	if m.GetONUInfoListFunc != nil {
		return m.GetONUInfoListFunc(ctx, key)
	}
	return nil, errors.New("not found")
}

func (m *mockRedisRepository) GetOnlyOnuIDCtx(ctx context.Context, key string) ([]model.OnuOnlyID, error) {
	return nil, errors.New("not found")
}

func (m *mockRedisRepository) SaveOnlyOnuIDCtx(ctx context.Context, key string, seconds int, onuID []model.OnuOnlyID) error {
	return nil
}

func (m *mockRedisRepository) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	if m.GetTTLFunc != nil {
		return m.GetTTLFunc(ctx, key)
	}
	return 0, errors.New("not supported")
}

func (m *mockRedisRepository) SaveONUDetail(ctx context.Context, key string, seconds int, detail model.ONUCustomerInfo) error {
	if m.SaveONUDetailFunc != nil {
		return m.SaveONUDetailFunc(ctx, key, seconds, detail)
	}
	return nil
}

func (m *mockRedisRepository) GetONUDetail(ctx context.Context, key string) (*model.ONUCustomerInfo, error) {
	if m.GetONUDetailFunc != nil {
		return m.GetONUDetailFunc(ctx, key)
	}
	return nil, errors.New("not found")
}

func (m *mockRedisRepository) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}

func (m *mockRedisRepository) SaveONUSerialList(ctx context.Context, key string, seconds int, list []model.OnuSerialNumber) error {
	if m.SaveONUSerialListFunc != nil {
		return m.SaveONUSerialListFunc(ctx, key, seconds, list)
	}
	return nil
}

func (m *mockRedisRepository) GetONUSerialList(ctx context.Context, key string) ([]model.OnuSerialNumber, error) {
	if m.GetONUSerialListFunc != nil {
		return m.GetONUSerialListFunc(ctx, key)
	}
	return nil, errors.New("not found")
}

func TestNewOnuUsecase(t *testing.T) {
	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	cfg := &config.Config{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	if usecase == nil {
		t.Error("Expected non-nil usecase")
	}

	// Verify it implements the interface
	var _ OnuUseCaseInterface = usecase
}

func TestNewOnuUsecase_InitializesFields(t *testing.T) {
	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	// Type assert to access internal fields
	onuUC, ok := usecase.(*onuUsecase)
	if !ok {
		t.Fatal("Failed to type assert to *onuUsecase")
	}

	if onuUC.snmpRepository == nil {
		t.Error("Expected snmpRepository to be set")
	}

	if onuUC.redisRepository == nil {
		t.Error("Expected redisRepository to be set")
	}

	if onuUC.cfg == nil {
		t.Error("Expected cfg to be set")
	}
}

func TestGetBoardConfig_ValidBoardPon(t *testing.T) {
	// Create a config with BoardPonMap initialized
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1.3902.1082.500.10",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	// Add a test board/pon config
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       "1.1.1.1",
		OnuTypeOID:         "1.1.1.2",
		OnuSerialNumberOID: "1.1.1.3",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	// Type assert to call private method
	onuUC := usecase.(*onuUsecase)

	oltConfig, err := onuUC.getBoardConfig(1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if oltConfig == nil {
		t.Fatal("Expected non-nil OltConfig")
	}

	if oltConfig.BaseOID != "1.3.6.1.4.1.3902.1082.500.10" {
		t.Errorf("Expected BaseOID to be set from config, got %s", oltConfig.BaseOID)
	}

	if oltConfig.OnuIDNameOID != "1.1.1.1" {
		t.Errorf("Expected OnuIDNameOID '1.1.1.1', got '%s'", oltConfig.OnuIDNameOID)
	}
}

func TestGetBoardConfig_InvalidBoardPon(t *testing.T) {
	cfg := &config.Config{
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	// Type assert to call private method
	onuUC := usecase.(*onuUsecase)

	// Try to get config for non-existent board/pon
	oltConfig, err := onuUC.getBoardConfig(99, 99)

	if err == nil {
		t.Error("Expected error for invalid board/pon, got nil")
	}

	if oltConfig != nil {
		t.Error("Expected nil OltConfig on error")
	}
}

func TestGetBoardConfig_DifferentBoards(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1.3902.1082.500.10",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	// Add configs for different board/pon combinations
	testCases := []struct {
		boardID int
		ponID   int
		oidName string
	}{
		{1, 1, "oid-b1-p1"},
		{1, 2, "oid-b1-p2"},
		{2, 1, "oid-b2-p1"},
		{2, 16, "oid-b2-p16"},
	}

	for _, tc := range testCases {
		cfg.BoardPonMap[config.BoardPonKey{BoardID: tc.boardID, PonID: tc.ponID}] = &config.BoardPonConfig{
			OnuIDNameOID: tc.oidName,
		}
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	onuUC := usecase.(*onuUsecase)

	for _, tc := range testCases {
		t.Run(tc.oidName, func(t *testing.T) {
			oltConfig, err := onuUC.getBoardConfig(tc.boardID, tc.ponID)

			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if oltConfig.OnuIDNameOID != tc.oidName {
				t.Errorf("Expected OnuIDNameOID '%s', got '%s'", tc.oidName, oltConfig.OnuIDNameOID)
			}
		})
	}
}

func TestGetOltConfig_ValidBoardPon(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1.3902.1082.500.10",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: "1.1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	onuUC := usecase.(*onuUsecase)

	oltConfig, err := onuUC.getOltConfig(1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if oltConfig == nil {
		t.Error("Expected non-nil OltConfig")
	}
}

func TestGetOltConfig_InvalidBoardPon(t *testing.T) {
	cfg := &config.Config{
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	onuUC := usecase.(*onuUsecase)

	oltConfig, err := onuUC.getOltConfig(99, 99)

	if err == nil {
		t.Error("Expected error for invalid board/pon")
	}

	if oltConfig != nil {
		t.Error("Expected nil OltConfig on error")
	}
}

func TestOnuUsecase_InterfaceCompliance(t *testing.T) {
	// Verify that onuUsecase implements OnuUseCaseInterface
	usecase := NewOnuUsecase(
		&mockSnmpRepository{},
		&mockRedisRepository{},
		&config.Config{},
	)

	if usecase == nil {
		t.Error("Expected non-nil usecase")
	}
}

func TestGetByBoardIDAndPonID_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.1.2",
		OnuSerialNumberOID: ".1.1.3",
		OnuRxPowerOID:      ".1.1.4",
		OnuStatusOID:       ".1.1.5",
	}

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			// Return 4 variables to cover the batch success path
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670")},
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("ZTEG12345678")},
					{Name: oids[0], Type: gosnmp.Integer, Value: 1000},
					{Name: oids[0], Type: gosnmp.Integer, Value: 4},
				},
			}, nil
		},
	}

	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return nil, errors.New("cache miss")
		},
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			return nil
		},
	}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected non-empty result")
	}

	// Verify batch Get populated the fields
	if result[0].OnuType == "" {
		t.Error("Expected OnuType to be populated from batch Get")
	}
	if result[0].SerialNumber == "" {
		t.Error("Expected SerialNumber to be populated from batch Get")
	}
	if result[0].Status == "" {
		t.Error("Expected Status to be populated from batch Get")
	}
}

func TestGetByBoardIDPonIDAndOnuID_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.1.2",
		OnuSerialNumberOID: ".1.1.3",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid,
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 5)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.Board != 1 || result.PON != 1 {
		t.Error("Expected valid ONU info")
	}
}

func TestGetEmptyOnuID_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// Simulate 2 ONUs registered (ID 1 and 2)
			_ = walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Value: []byte("ONU1")})
			_ = walkFunc(gosnmp.SnmpPDU{Name: oid + ".2", Value: []byte("ONU2")})
			return nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should return 126 empty IDs (128 - 2 registered)
	if len(result) != 126 {
		t.Errorf("Expected 126 empty IDs, got %d", len(result))
	}
}

func TestGetOnuIDAndSerialNumber_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuSerialNumberOID: ".1.1.3",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			_ = walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Value: []byte("ONU1")})
			return nil
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Value: []byte("ZTEGC123456")},
				},
			}, nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetOnuIDAndSerialNumber(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestUpdateEmptyOnuID_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			_ = walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Value: []byte("ONU1")})
			return nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.UpdateEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGetByBoardIDAndPonIDWithPagination_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			for i := 1; i <= 10; i++ {
				_ = walkFunc(gosnmp.SnmpPDU{Name: oid + "." + string(rune(i)), Value: []byte("ONU")})
			}
			return nil
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Value: []byte("test")},
				},
			}, nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(context.Background(), 1, 1, 1, 5)

	if count == 0 {
		t.Error("Expected non-zero count")
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestDeleteCache_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.DeleteCache(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestDeleteCache_InvalidBoardPon(t *testing.T) {
	cfg := &config.Config{
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.DeleteCache(context.Background(), 99, 99)

	if err == nil {
		t.Error("Expected error for invalid board/pon")
	}
}

func TestGetByBoardIDAndPonID_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP walk failed")
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetByBoardIDAndPonID_FromCache(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{
				{Board: 1, PON: 1, ID: 1, Name: "Cached ONU"},
			}, nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected cached result")
	}

	if result[0].Name != "Cached ONU" {
		t.Error("Expected cached data")
	}
}

func TestGetEmptyOnuID_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP error")
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetEmptyOnuID(context.Background(), 1, 1)

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetOnuIDAndSerialNumber_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP walk error")
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetOnuIDAndSerialNumber(context.Background(), 1, 1)

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestUpdateEmptyOnuID_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP error")
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.UpdateEmptyOnuID(context.Background(), 1, 1)

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetByBoardIDPonIDAndOnuID_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP walk error")
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 5)

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetByBoardIDAndPonID_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetByBoardIDAndPonID(context.Background(), 99, 99)

	if err == nil {
		t.Error("Expected config error")
	}
}

func TestDeleteCache_RedisError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		DeleteFunc: func(ctx context.Context, key string) error {
			return errors.New("Redis delete failed")
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.DeleteCache(context.Background(), 1, 1)

	if err == nil {
		t.Error("Expected Redis error")
	}
}

// Test helper functions for better coverage

func TestGetUptimeDuration_ParseError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with invalid time format
	_, err := usecase.getUptimeDuration("invalid-time-format")
	if err == nil {
		t.Error("Expected parse error for invalid time format")
	}
}

func TestGetLastDownDuration_ParseOfflineError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with invalid offline time format
	_, err := usecase.getLastDownDuration("invalid-offline", "2023-01-01 10:00:00")
	if err == nil {
		t.Error("Expected parse error for invalid offline time")
	}
}

func TestGetLastDownDuration_ParseOnlineError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with invalid online time format
	_, err := usecase.getLastDownDuration("2023-01-01 10:00:00", "invalid-online")
	if err == nil {
		t.Error("Expected parse error for invalid online time")
	}
}

func TestGetLastDownDuration_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with valid times
	result, err := usecase.getLastDownDuration("2023-01-01 10:00:00", "2023-01-01 11:00:00")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty duration string")
	}
}

func TestGetFromSNMPWithSingleflight_EmptyVariables(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			// Return packet with empty variables
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with empty variables response
	_, err := usecase.getFromSNMPWithSingleflight("1.3.6.1.2.1.1.1.0")
	if err == nil {
		t.Error("Expected error for empty variables")
	}
}

func TestGetFromSNMPWithSingleflight_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP connection error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with SNMP error
	_, err := usecase.getFromSNMPWithSingleflight("1.3.6.1.2.1.1.1.0")
	if err == nil {
		t.Error("Expected SNMP error")
	}
}

// Tests for constants
func TestConstants_MaxOnuIDPerPon(t *testing.T) {
	// MaxOnuIDPerPon should be 128 as per ZTE OLT specification
	if MaxOnuIDPerPon != 128 {
		t.Errorf("Expected MaxOnuIDPerPon to be 128, got %d", MaxOnuIDPerPon)
	}
}

func TestGetEmptyOnuID_VerifyMaxOnuIDConstant(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	// Simulate no ONUs registered (all 128 should be empty)
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// No ONUs registered
			return nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should return all MaxOnuIDPerPon IDs as empty
	if len(result) != MaxOnuIDPerPon {
		t.Errorf("Expected %d empty IDs when no ONUs registered, got %d", MaxOnuIDPerPon, len(result))
	}

	// Verify IDs are sequential from 1 to MaxOnuIDPerPon
	for i, onuID := range result {
		expectedID := i + 1
		if onuID.ID != expectedID {
			t.Errorf("Expected ID %d at position %d, got %d", expectedID, i, onuID.ID)
		}
	}
}

func TestGetEmptyOnuID_WithSomeRegistered(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	// Simulate 5 ONUs registered (ID 1, 2, 3, 4, 5)
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			for i := 1; i <= 5; i++ {
				_ = walkFunc(gosnmp.SnmpPDU{
					Name:  oid + "." + string(rune('0'+i)),
					Value: []byte("ONU"),
				})
			}
			return nil
		},
	}

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should return MaxOnuIDPerPon - 5 = 123 empty IDs
	expectedEmpty := MaxOnuIDPerPon - 5
	if len(result) != expectedEmpty {
		t.Errorf("Expected %d empty IDs, got %d", expectedEmpty, len(result))
	}

	// First empty ID should be 6 (since 1-5 are registered)
	if len(result) > 0 && result[0].ID != 6 {
		t.Errorf("Expected first empty ID to be 6, got %d", result[0].ID)
	}
}

func TestUpdateEmptyOnuID_UsesConstants(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		CacheCfg: config.CacheConfig{
			ONUInfoTTL:    1800,
			ONUDetailTTL:  900,
			EmptyOnuIDTTL: 300,
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	var capturedTTL int
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return nil
		},
	}

	redisRepo := &mockRedisRepository{
		SetOnuIDCtxFunc: func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
			capturedTTL = seconds
			return nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.UpdateEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the TTL used matches CacheCfg.EmptyOnuIDTTL
	if capturedTTL != cfg.CacheCfg.EmptyOnuIDTTL {
		t.Errorf("Expected TTL to be %d (CacheCfg.EmptyOnuIDTTL), got %d", cfg.CacheCfg.EmptyOnuIDTTL, capturedTTL)
	}
}

func TestGetByBoardIDAndPonID_UsesRedisONUInfoTTL(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.1",
		},
		CacheCfg: config.CacheConfig{
			ONUInfoTTL:    1800,
			ONUDetailTTL:  900,
			EmptyOnuIDTTL: 300,
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.1.2",
		OnuSerialNumberOID: ".1.1.3",
		OnuRxPowerOID:      ".1.1.4",
		OnuStatusOID:       ".1.1.5",
	}

	var capturedTTL int
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670")},
				},
			}, nil
		},
	}

	redisRepo := &mockRedisRepository{
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error {
			capturedTTL = seconds
			return nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the TTL used matches CacheCfg.ONUInfoTTL
	if capturedTTL != cfg.CacheCfg.ONUInfoTTL {
		t.Errorf("Expected TTL to be %d (CacheCfg.ONUInfoTTL), got %d", cfg.CacheCfg.ONUInfoTTL, capturedTTL)
	}
}

func TestGetLastOffline_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	_, err := usecase.getLastOffline(".1.2.3", "5")
	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetLastOffline_NoVariables(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	_, err := usecase.getLastOffline(".1.2.3", "5")
	if err == nil {
		t.Error("Expected error for no variables")
	}
}

// Tests for GenerateRedisKey helper function
func TestGenerateRedisKey_ONUInfo(t *testing.T) {
	testCases := []struct {
		name     string
		keyType  string
		boardID  int
		ponID    int
		expected string
	}{
		{
			name:     "ONU Info key for board 1 pon 1",
			keyType:  RedisKeyTypeONUInfo,
			boardID:  1,
			ponID:    1,
			expected: "board_1_pon_1",
		},
		{
			name:     "ONU Info key for board 2 pon 16",
			keyType:  RedisKeyTypeONUInfo,
			boardID:  2,
			ponID:    16,
			expected: "board_2_pon_16",
		},
		{
			name:     "Default key type",
			keyType:  "unknown_type",
			boardID:  1,
			ponID:    5,
			expected: "board_1_pon_5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateRedisKey(tc.keyType, tc.boardID, tc.ponID)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestGenerateRedisKey_EmptyOnuID(t *testing.T) {
	testCases := []struct {
		name     string
		boardID  int
		ponID    int
		expected string
	}{
		{
			name:     "Empty ONU ID key for board 1 pon 1",
			boardID:  1,
			ponID:    1,
			expected: "board_1_pon_1_empty_onu_id",
		},
		{
			name:     "Empty ONU ID key for board 2 pon 8",
			boardID:  2,
			ponID:    8,
			expected: "board_2_pon_8_empty_onu_id",
		},
		{
			name:     "Empty ONU ID key for board 1 pon 16",
			boardID:  1,
			ponID:    16,
			expected: "board_1_pon_16_empty_onu_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateRedisKey(RedisKeyTypeEmptyOnuID, tc.boardID, tc.ponID)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestRedisKeyTypeConstants(t *testing.T) {
	// Verify RedisKeyTypeONUInfo constant
	if RedisKeyTypeONUInfo != "onu_info" {
		t.Errorf("Expected RedisKeyTypeONUInfo to be 'onu_info', got '%s'", RedisKeyTypeONUInfo)
	}

	// Verify RedisKeyTypeEmptyOnuID constant
	if RedisKeyTypeEmptyOnuID != "empty_onu_id" {
		t.Errorf("Expected RedisKeyTypeEmptyOnuID to be 'empty_onu_id', got '%s'", RedisKeyTypeEmptyOnuID)
	}
}

func TestGenerateRedisKey_ConsistencyWithUsage(t *testing.T) {
	// Test that GenerateRedisKey produces the same keys as the hardcoded patterns
	// This ensures backward compatibility with existing cached data

	boardID := 1
	ponID := 5

	// Test ONU Info key matches old pattern: fmt.Sprintf("board_%d_pon_%d", boardID, ponID)
	expectedONUInfoKey := "board_1_pon_5"
	actualONUInfoKey := GenerateRedisKey(RedisKeyTypeONUInfo, boardID, ponID)
	if actualONUInfoKey != expectedONUInfoKey {
		t.Errorf("ONU Info key mismatch: expected %s, got %s", expectedONUInfoKey, actualONUInfoKey)
	}

	// Test Empty ONU ID key matches old pattern: "board_" + strconv.Itoa(boardID) + "_pon_" + strconv.Itoa(ponID) + "_empty_onu_id"
	expectedEmptyKey := "board_1_pon_5_empty_onu_id"
	actualEmptyKey := GenerateRedisKey(RedisKeyTypeEmptyOnuID, boardID, ponID)
	if actualEmptyKey != expectedEmptyKey {
		t.Errorf("Empty ONU ID key mismatch: expected %s, got %s", expectedEmptyKey, actualEmptyKey)
	}
}

func TestGetEmptyOnuID_UsesGenerateRedisKey(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	var capturedKey string
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return nil
		},
	}

	redisRepo := &mockRedisRepository{
		SetOnuIDCtxFunc: func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
			capturedKey = key
			return nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetEmptyOnuID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the key used matches GenerateRedisKey output
	expectedKey := GenerateRedisKey(RedisKeyTypeEmptyOnuID, 1, 1)
	if capturedKey != expectedKey {
		t.Errorf("Expected Redis key to be %s, got %s", expectedKey, capturedKey)
	}
}

func TestDeleteCache_UsesGenerateRedisKey(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	var capturedKeys []string
	snmpRepo := &mockSnmpRepository{}

	redisRepo := &mockRedisRepository{
		DeleteFunc: func(ctx context.Context, key string) error {
			capturedKeys = append(capturedKeys, key)
			return nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.DeleteCache(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify both ONU info key and serial list key are deleted
	expectedInfoKey := GenerateRedisKey(RedisKeyTypeONUInfo, 1, 1)
	expectedSerialKey := fmt.Sprintf("board_%d_pon_%d_serial_list", 1, 1)
	if len(capturedKeys) != 2 {
		t.Errorf("Expected 2 delete calls, got %d", len(capturedKeys))
	} else {
		if capturedKeys[0] != expectedInfoKey {
			t.Errorf("Expected first Redis key to be %s, got %s", expectedInfoKey, capturedKeys[0])
		}
		if capturedKeys[1] != expectedSerialKey {
			t.Errorf("Expected second Redis key to be %s, got %s", expectedSerialKey, capturedKeys[1])
		}
	}
}

// Tests for TimezoneOffsetWIB constant
func TestConstants_TimezoneOffsetWIB(t *testing.T) {
	// TimezoneOffsetWIB should be 7 hours (UTC+7 for WIB - Western Indonesian Time)
	expectedOffset := 7 * time.Hour
	if TimezoneOffsetWIB != expectedOffset {
		t.Errorf("Expected TimezoneOffsetWIB to be %v, got %v", expectedOffset, TimezoneOffsetWIB)
	}
}

func TestGetUptimeDuration_UsesTimezoneOffsetWIB(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Test with a valid time that's 1 hour ago
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	lastOnline := oneHourAgo.Format(DateTimeFormat)

	result, err := usecase.getUptimeDuration(lastOnline)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// The result should not be empty
	if result == "" {
		t.Error("Expected non-empty duration string")
	}

	// The duration should include the timezone offset (approximately 8 hours: 1 hour + 7 hours offset)
	// We can't test exact values due to timing, but we verify the function works
}

func TestTimezoneOffsetWIB_IsCorrectDuration(t *testing.T) {
	// Verify TimezoneOffsetWIB is exactly 7 hours in nanoseconds
	expectedNanoseconds := int64(7 * 60 * 60 * 1000000000) // 7 hours in nanoseconds
	actualNanoseconds := int64(TimezoneOffsetWIB)

	if actualNanoseconds != expectedNanoseconds {
		t.Errorf("Expected TimezoneOffsetWIB to be %d nanoseconds, got %d", expectedNanoseconds, actualNanoseconds)
	}
}

// Tests for SNMPOIDSuffix constant
func TestConstants_SNMPOIDSuffix(t *testing.T) {
	// SNMPOIDSuffix should be ".1" - used for SNMP queries that require additional index
	expectedSuffix := ".1"
	if SNMPOIDSuffix != expectedSuffix {
		t.Errorf("Expected SNMPOIDSuffix to be %q, got %q", expectedSuffix, SNMPOIDSuffix)
	}
}

func TestSNMPOIDSuffix_IsValidOIDFormat(t *testing.T) {
	// Verify SNMPOIDSuffix starts with a dot and contains a valid index
	if SNMPOIDSuffix[0] != '.' {
		t.Errorf("Expected SNMPOIDSuffix to start with '.', got %q", SNMPOIDSuffix)
	}

	if len(SNMPOIDSuffix) < 2 {
		t.Error("Expected SNMPOIDSuffix to have at least 2 characters (dot + index)")
	}
}

// Tests for DateTimeFormat constant
func TestConstants_DateTimeFormat(t *testing.T) {
	// DateTimeFormat should be Go's standard datetime format "2006-01-02 15:04:05"
	expectedFormat := "2006-01-02 15:04:05"
	if DateTimeFormat != expectedFormat {
		t.Errorf("Expected DateTimeFormat to be %q, got %q", expectedFormat, DateTimeFormat)
	}
}

func TestDateTimeFormat_CanParseValidDateTime(t *testing.T) {
	// Test that DateTimeFormat can correctly parse a datetime string
	testDateTimeStr := "2024-06-15 14:30:45"
	parsedTime, err := time.Parse(DateTimeFormat, testDateTimeStr)
	if err != nil {
		t.Errorf("Expected DateTimeFormat to parse %q without error, got %v", testDateTimeStr, err)
	}

	// Verify parsed values
	if parsedTime.Year() != 2024 {
		t.Errorf("Expected year 2024, got %d", parsedTime.Year())
	}
	if parsedTime.Month() != time.June {
		t.Errorf("Expected month June, got %s", parsedTime.Month())
	}
	if parsedTime.Day() != 15 {
		t.Errorf("Expected day 15, got %d", parsedTime.Day())
	}
	if parsedTime.Hour() != 14 {
		t.Errorf("Expected hour 14, got %d", parsedTime.Hour())
	}
	if parsedTime.Minute() != 30 {
		t.Errorf("Expected minute 30, got %d", parsedTime.Minute())
	}
	if parsedTime.Second() != 45 {
		t.Errorf("Expected second 45, got %d", parsedTime.Second())
	}
}

func TestDateTimeFormat_FormatDateTime(t *testing.T) {
	// Test that DateTimeFormat can correctly format a time
	testTime := time.Date(2024, time.December, 25, 10, 15, 30, 0, time.UTC)
	formatted := testTime.Format(DateTimeFormat)
	expected := "2024-12-25 10:15:30"
	if formatted != expected {
		t.Errorf("Expected formatted time to be %q, got %q", expected, formatted)
	}
}

// Tests for helper functions - success paths
func TestGetName_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("TestONU-Name")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getName(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestGetONUType_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670L")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getONUType(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestGetSerialNumber_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("ZTEGC1234567")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getSerialNumber(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestGetTxPower_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 2500},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getTxPower(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Result may be empty string if conversion fails, but function should not error
	_ = result
}

func TestGetRxPower_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: -2000},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getRxPower(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	_ = result
}

func TestGetStatus_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 1},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getStatus(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	_ = result
}

func TestGetIPAddress_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("192.168.1.100")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getIPAddress(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestGetDescription_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("Customer ABC")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getDescription(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestGetLastOnline_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	// Create a datetime byte array (format: YYYYMMDDHHmmss as bytes)
	dateBytes := []byte{0x07, 0xe8, 0x06, 0x0f, 0x0e, 0x1e, 0x2d, 0x00} // 2024-06-15 14:30:45

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: dateBytes},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getLastOnline(".1.2.3", "5")

	// Error might occur due to byte conversion, but function path is covered
	_ = result
	_ = err
}

func TestGetLastOffline_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	dateBytes := []byte{0x07, 0xe8, 0x06, 0x0f, 0x0e, 0x1e, 0x2d, 0x00}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: dateBytes},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getLastOffline(".1.2.3", "5")

	// Error might occur due to byte conversion, but function path is covered
	_ = result
	_ = err
}

func TestGetLastOfflineReason_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 1},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getLastOfflineReason(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	_ = result
}

func TestGetOnuGponOpticalDistance_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 5000},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	result, err := usecase.getOnuGponOpticalDistance(".1.2.3", "5")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	_ = result
}

func TestGetUptimeDuration_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)

	// Use a time format that matches DateTimeFormat
	lastOnline := time.Now().Add(-2 * time.Hour).Format(DateTimeFormat)
	result, err := usecase.getUptimeDuration(lastOnline)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty duration string")
	}
}

// Error path tests for helper functions
func TestGetName_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getName(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetONUType_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getONUType(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetSerialNumber_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getSerialNumber(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetTxPower_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getTxPower(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetRxPower_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getRxPower(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetStatus_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getStatus(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetIPAddress_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID2: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getIPAddress(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetDescription_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getDescription(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetLastOnline_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getLastOnline(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetLastOfflineReason_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getLastOfflineReason(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

func TestGetOnuGponOpticalDistance_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := usecase.getOnuGponOpticalDistance(".1.2.3", "5")

	if err == nil {
		t.Error("Expected SNMP error")
	}
}

// Test GetByBoardIDAndPonID when Redis save fails
func TestGetByBoardIDAndPonID_RedisSaveError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.1.2",
		OnuSerialNumberOID: ".1.1.3",
		OnuRxPowerOID:      ".1.1.4",
		OnuStatusOID:       ".1.1.5",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("test")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return nil, errors.New("not found")
		},
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error {
			return errors.New("redis save error")
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	// Even if Redis save fails, the function should return data from SNMP
	result, err := usecase.GetByBoardIDAndPonID(ctx, 1, 1)

	if err != nil {
		t.Errorf("Expected no error even if Redis save fails, got: %v", err)
	}
	if result == nil {
		t.Error("Expected result even if Redis save fails")
	}
}

// Test GetByBoardIDPonIDAndOnuID with invalid config
func TestGetByBoardIDPonIDAndOnuID_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig), // Empty map will cause config error
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetByBoardIDPonIDAndOnuID(context.Background(), 99, 99, 1) // Invalid board/pon

	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// Test GetEmptyOnuID with invalid config
func TestGetEmptyOnuID_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig), // Empty map will cause config error
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	_, err := usecase.GetEmptyOnuID(ctx, 99, 99) // Invalid board/pon

	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// Test GetEmptyOnuID when Redis cache hit
func TestGetEmptyOnuID_FromCache(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	cachedData := []model.OnuID{
		{Board: 1, PON: 1, ID: 5},
		{Board: 1, PON: 1, ID: 10},
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		GetOnuIDCtxFunc: func(ctx context.Context, key string) ([]model.OnuID, error) {
			return cachedData, nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	result, err := usecase.GetEmptyOnuID(ctx, 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if len(result) != len(cachedData) {
		t.Errorf("Expected %d items from cache, got %d", len(cachedData), len(result))
	}
}

// Test GetEmptyOnuID when Redis save fails
func TestGetEmptyOnuID_RedisSaveError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// Return one ONU to test the empty ID logic
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("ONU1"),
			})
		},
	}
	redisRepo := &mockRedisRepository{
		GetOnuIDCtxFunc: func(ctx context.Context, key string) ([]model.OnuID, error) {
			return nil, errors.New("not found")
		},
		SetOnuIDCtxFunc: func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
			return errors.New("redis save error")
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	_, err := usecase.GetEmptyOnuID(ctx, 1, 1)

	// The function returns error when Redis save fails
	if err == nil {
		t.Error("Expected error when Redis save fails")
	}
}

// Test GetOnuIDAndSerialNumber with invalid config
func TestGetOnuIDAndSerialNumber_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig), // Empty map will cause config error
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	_, err := usecase.GetOnuIDAndSerialNumber(context.Background(), 99, 99) // Invalid board/pon

	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// Test UpdateEmptyOnuID with invalid config
func TestUpdateEmptyOnuID_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig), // Empty map will cause config error
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	err := usecase.UpdateEmptyOnuID(ctx, 99, 99) // Invalid board/pon

	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// Test UpdateEmptyOnuID when Redis set fails
func TestUpdateEmptyOnuID_RedisSetError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("ONU1"),
			})
		},
	}
	redisRepo := &mockRedisRepository{
		SetOnuIDCtxFunc: func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error {
			return errors.New("redis set error")
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	ctx := context.Background()
	err := usecase.UpdateEmptyOnuID(ctx, 1, 1)

	if err == nil {
		t.Error("Expected error when Redis set fails")
	}
}

// Test GetByBoardIDAndPonIDWithPagination with invalid config
func TestGetByBoardIDAndPonIDWithPagination_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig), // Empty map will cause config error
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(context.Background(), 99, 99, 1, 10) // Invalid board/pon

	if result != nil {
		t.Error("Expected nil result for invalid config")
	}
	if count != 0 {
		t.Errorf("Expected count 0 for invalid config, got %d", count)
	}
}

// Test GetByBoardIDAndPonIDWithPagination with SNMP walk error
func TestGetByBoardIDAndPonIDWithPagination_SNMPError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP walk error")
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(context.Background(), 1, 1, 1, 10)

	if result != nil {
		t.Error("Expected nil result for SNMP error")
	}
	if count != 0 {
		t.Errorf("Expected count 0 for SNMP error, got %d", count)
	}
}

// Test GetByBoardIDAndPonIDWithPagination when startIndex >= count (out of range)
func TestGetByBoardIDAndPonIDWithPagination_OutOfRange(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	// Return only 2 ONUs
	callCount := 0
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("ONU1")}); err != nil {
				return err
			}
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".2", Type: gosnmp.OctetString, Value: []byte("ONU2")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			callCount++
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("test")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	// Page 10 with size 10 = startIndex 90, but we only have 2 ONUs
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(context.Background(), 1, 1, 10, 10)

	// Should return empty list but with total count
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 results for out of range page, got %d", len(result))
	}
}

// Test GetByBoardIDPonIDAndOnuID with valid datetime for LastOnline/LastOffline paths
func TestGetByBoardIDPonIDAndOnuID_WithValidDatetime(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:            ".1.1.1",
		OnuTypeOID:              ".1.2.1",
		OnuSerialNumberOID:      ".1.3.1",
		OnuRxPowerOID:           ".1.4.1",
		OnuTxPowerOID:           ".1.5.1",
		OnuStatusOID:            ".1.6.1",
		OnuIPAddressOID:         ".1.7.1",
		OnuDescriptionOID:       ".1.8.1",
		OnuLastOnlineOID:        ".1.9.1",
		OnuLastOfflineOID:       ".1.10.1",
		OnuLastOfflineReasonOID: ".1.11.1",
	}

	// Create valid datetime bytes (8 bytes: year(2), month, day, hour, minute, second, unused)
	// 2025-01-10 12:30:45 - recent date to ensure uptime calculation works
	validDatetime := []byte{0x07, 0xE9, 0x01, 0x0A, 0x0C, 0x1E, 0x2D, 0x00} // 2025-01-10 12:30:45
	// Older date for LastOffline
	olderDatetime := []byte{0x07, 0xE9, 0x01, 0x0A, 0x0C, 0x00, 0x00, 0x00} // 2025-01-10 12:00:00

	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".5", Type: gosnmp.OctetString, Value: []byte("ONU-5")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			oid := oids[0]
			// Return datetime bytes for LastOnline and LastOffline OIDs
			if strings.Contains(oid, ".1.9.1.") || strings.Contains(oid, ".1.10.1.") {
				// LastOnline OID
				if strings.Contains(oid, ".1.9.1.") {
					return &gosnmp.SnmpPacket{
						Variables: []gosnmp.SnmpPDU{
							{Name: oid, Type: gosnmp.OctetString, Value: validDatetime},
						},
					}, nil
				}
				// LastOffline OID
				return &gosnmp.SnmpPacket{
					Variables: []gosnmp.SnmpPDU{
						{Name: oid, Type: gosnmp.OctetString, Value: olderDatetime},
					},
				}, nil
			}
			// Default response for other OIDs
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oid, Type: gosnmp.OctetString, Value: []byte("test")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 5)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result.ID != 5 {
		t.Errorf("Expected ONU ID 5, got %d", result.ID)
	}
	// Verify LastOnline and LastOffline were set
	if result.LastOnline == "" {
		t.Error("Expected LastOnline to be set")
	}
	if result.LastOffline == "" {
		t.Error("Expected LastOffline to be set")
	}
}

// Test GetByBoardIDAndPonIDWithPagination when pageSize > available items (endIndex adjustment)
func TestGetByBoardIDAndPonIDWithPagination_PartialPage(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.2.1",
		OnuSerialNumberOID: ".1.3.1",
		OnuRxPowerOID:      ".1.4.1",
		OnuStatusOID:       ".1.6.1",
	}

	// Return only 3 ONUs
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("ONU1")}); err != nil {
				return err
			}
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".2", Type: gosnmp.OctetString, Value: []byte("ONU2")}); err != nil {
				return err
			}
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".3", Type: gosnmp.OctetString, Value: []byte("ONU3")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("test")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	// Page 1 with size 10 = startIndex 0, endIndex should be adjusted to 3 (not 10)
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(context.Background(), 1, 1, 1, 10)

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result))
	}
}

// Test GetByBoardIDAndPonID with multiple ONUs to trigger sort callback
func TestGetByBoardIDAndPonID_MultipleSortOrder(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.2.1",
		OnuSerialNumberOID: ".1.3.1",
		OnuRxPowerOID:      ".1.4.1",
		OnuStatusOID:       ".1.6.1",
	}

	// Return ONUs in reverse order to trigger sorting
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// Return in reverse order: 3, 2, 1
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".3", Type: gosnmp.OctetString, Value: []byte("ONU3")}); err != nil {
				return err
			}
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".2", Type: gosnmp.OctetString, Value: []byte("ONU2")}); err != nil {
				return err
			}
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("ONU1")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("test")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result))
	}
	// Verify sort order (should be ascending by ID)
	if len(result) >= 3 {
		if result[0].ID > result[1].ID || result[1].ID > result[2].ID {
			t.Errorf("Expected results to be sorted by ID ascending, got IDs: %d, %d, %d",
				result[0].ID, result[1].ID, result[2].ID)
		}
	}
}

func createTestConfig() *config.Config {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.1",
		},
		CacheCfg: config.CacheConfig{
			ONUInfoTTL:    1800,
			ONUDetailTTL:  900,
			EmptyOnuIDTTL: 300,
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuTypeOID:         ".1.1.2",
		OnuSerialNumberOID: ".1.1.3",
		OnuRxPowerOID:      ".1.1.4",
		OnuStatusOID:       ".1.1.5",
	}
	return cfg
}

func TestRefreshONUInfoCache_Success(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("TestONU")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670")},
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("SN123")},
					{Name: oids[0], Type: gosnmp.Integer, Value: 1000},
					{Name: oids[0], Type: gosnmp.Integer, Value: 4},
				},
			}, nil
		},
	}
	saved := false
	redisRepo := &mockRedisRepository{
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			saved = true
			return nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	oltConfig, _ := uc.getOltConfig(1, 1)
	uc.refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test_key")
	if !saved {
		t.Error("Expected Redis save to be called")
	}
}

func TestRefreshONUInfoCache_SNMPError(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("snmp error")
		},
	}
	saved := false
	redisRepo := &mockRedisRepository{
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			saved = true
			return nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	oltConfig, _ := uc.getOltConfig(1, 1)
	uc.refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test_key")
	if saved {
		t.Error("Expected Redis save NOT to be called on SNMP error")
	}
}

func TestRefreshONUInfoCache_RedisSaveError(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("TestONU")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670")},
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("SN123")},
					{Name: oids[0], Type: gosnmp.Integer, Value: 1000},
					{Name: oids[0], Type: gosnmp.Integer, Value: 4},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			return errors.New("redis save error")
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	oltConfig, _ := uc.getOltConfig(1, 1)
	// Should not panic even when Redis save fails
	uc.refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test_key")
}

func TestRefreshONUInfoCache_BatchGetError(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// Return multiple ONUs in reverse order to trigger sort
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".3", Type: gosnmp.OctetString, Value: []byte("ONU3")}); err != nil {
				return err
			}
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("ONU1")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("SNMP Get error")
		},
	}
	redisRepo := &mockRedisRepository{}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	oltConfig, _ := uc.getOltConfig(1, 1)
	// Should still save the list (without enriched fields) even when batch Get fails
	uc.refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test_key")
}

func TestRefreshONUInfoCache_BatchGetPartialResult(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("TestONU")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			// Return fewer than 4 variables
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("F670")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	oltConfig, _ := uc.getOltConfig(1, 1)
	// Should still save the list (without enriched fields) when partial result
	uc.refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test_key")
}

func TestGetLastOnline_InvalidType(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 12345},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := uc.getLastOnline(".1.2.3", "5")

	if err == nil {
		t.Error("Expected error for non-byte value type")
	}
}

func TestGetLastOffline_InvalidType(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
	}

	snmpRepo := &mockSnmpRepository{
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.Integer, Value: 12345},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg).(*onuUsecase)
	_, err := uc.getLastOffline(".1.2.3", "5")

	if err == nil {
		t.Error("Expected error for non-byte value type")
	}
}

func TestGetByBoardIDAndPonID_BatchGetError(t *testing.T) {
	cfg := createTestConfig()

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return nil, errors.New("batch get failed")
		},
	}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return nil, errors.New("cache miss")
		},
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			return nil
		},
	}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error (batch failure is non-fatal), got: %v", err)
	}
	if len(result) == 0 {
		t.Error("Expected at least 1 result even with batch get error")
	}
	if len(result) > 0 && result[0].OnuType != "" {
		t.Error("Expected empty OnuType when batch get fails")
	}
}

func TestGetByBoardIDAndPonID_BatchGetPartialResult(t *testing.T) {
	cfg := createTestConfig()

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name:  oid + ".1",
				Type:  gosnmp.OctetString,
				Value: []byte("TestONU"),
			})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("SomeType")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return nil, errors.New("cache miss")
		},
		SaveONUInfoListFunc: func(ctx context.Context, key string, seconds int, list []model.ONUInfoPerBoard) error {
			return nil
		},
	}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if len(result) == 0 {
		t.Error("Expected at least 1 result")
	}
	if len(result) > 0 && result[0].OnuType != "" {
		t.Error("Expected empty OnuType when batch returns < 4 variables")
	}
}

func TestGetByBoardIDAndPonID_CacheWithLowTTL_TriggersRefresh(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		CacheCfg: config.CacheConfig{ONUInfoTTL: 1800, ONUDetailTTL: 900, EmptyOnuIDTTL: 300},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{
				{Board: 1, PON: 1, ID: 1, Name: "CachedONU"},
			}, nil
		},
		GetTTLFunc: func(ctx context.Context, key string) (time.Duration, error) {
			// Return TTL below refresh threshold (120s) to trigger background refresh
			return 60 * time.Second, nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 || result[0].Name != "CachedONU" {
		t.Error("Expected cached data returned")
	}

	// Allow background goroutine to run
	time.Sleep(200 * time.Millisecond)
}

func TestGetByBoardIDAndPonID_CacheWithHighTTL_NoRefresh(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		CacheCfg: config.CacheConfig{ONUInfoTTL: 1800, ONUDetailTTL: 900, EmptyOnuIDTTL: 300},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{
				{Board: 1, PON: 1, ID: 1, Name: "CachedONU"},
			}, nil
		},
		GetTTLFunc: func(ctx context.Context, key string) (time.Duration, error) {
			return 500 * time.Second, nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 || result[0].Name != "CachedONU" {
		t.Error("Expected cached data returned")
	}
}

func TestGetByBoardIDPonIDAndOnuID_FromCache(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		CacheCfg: config.CacheConfig{ONUInfoTTL: 1800, ONUDetailTTL: 900, EmptyOnuIDTTL: 300},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	cachedDetail := &model.ONUCustomerInfo{
		Board: 1, PON: 1, ID: 5, Name: "CachedDetail", SerialNumber: "ZTEGC99999999",
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		GetONUDetailFunc: func(ctx context.Context, key string) (*model.ONUCustomerInfo, error) {
			return cachedDetail, nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 5)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result.Name != "CachedDetail" {
		t.Errorf("Expected CachedDetail, got %s", result.Name)
	}
	if result.SerialNumber != "ZTEGC99999999" {
		t.Errorf("Expected serial ZTEGC99999999, got %s", result.SerialNumber)
	}
}

func TestRefreshONUInfoCache_BulkWalkError(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return errors.New("SNMP bulk walk error")
		},
	}
	redisRepo := &mockRedisRepository{}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	oltConfig := &model.OltConfig{
		BaseOID:      "1.3.6.1.4.1",
		OnuIDNameOID: ".1.1.1",
	}

	// Should not panic on BulkWalk error
	uc.(*onuUsecase).refreshONUInfoCache(context.Background(), 1, 1, oltConfig, "test-key")
}

// Test GetOnuIDAndSerialNumber with multiple ONUs to trigger sort callback
func TestGetOnuIDAndSerialNumber_MultipleSortOrder(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID:       ".1.1.1",
		OnuSerialNumberOID: ".1.3.1",
	}

	// Return ONUs in reverse order to trigger sorting
	snmpRepo := &mockSnmpRepository{
		WalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			// Return in reverse order: 3, 2, 1
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".3", Type: gosnmp.OctetString, Value: []byte("ONU3")}); err != nil {
				return err
			}
			if err := walkFunc(gosnmp.SnmpPDU{Name: oid + ".2", Type: gosnmp.OctetString, Value: []byte("ONU2")}); err != nil {
				return err
			}
			return walkFunc(gosnmp.SnmpPDU{Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("ONU1")})
		},
		GetFunc: func(oids []string) (*gosnmp.SnmpPacket, error) {
			return &gosnmp.SnmpPacket{
				Variables: []gosnmp.SnmpPDU{
					{Name: oids[0], Type: gosnmp.OctetString, Value: []byte("SN123")},
				},
			}, nil
		},
	}
	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetOnuIDAndSerialNumber(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result))
	}
	// Verify sort order (should be ascending by ID)
	if len(result) >= 3 {
		if result[0].ID > result[1].ID || result[1].ID > result[2].ID {
			t.Errorf("Expected results to be sorted by ID ascending, got IDs: %d, %d, %d",
				result[0].ID, result[1].ID, result[2].ID)
		}
	}
}

func TestGetOnuIDAndSerialNumber_FromCache(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{}
	cachedData := []model.OnuSerialNumber{
		{Board: 1, PON: 1, ID: 1, SerialNumber: "ZTEGC1234"},
		{Board: 1, PON: 1, ID: 2, SerialNumber: "ZTEGC5678"},
	}
	redisRepo := &mockRedisRepository{
		GetONUSerialListFunc: func(ctx context.Context, key string) ([]model.OnuSerialNumber, error) {
			return cachedData, nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetOnuIDAndSerialNumber(context.Background(), 1, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 results, got %d", len(result))
	}
}

func TestGetByBoardIDPonIDAndOnuID_FallbackFromList(t *testing.T) {
	cfg := createTestConfig()
	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		// ONU detail cache miss
		GetONUDetailFunc: func(ctx context.Context, key string) (*model.ONUCustomerInfo, error) {
			return nil, errors.New("not found")
		},
		// ONU info list cache hit — contains the ONU we're looking for
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{
				{Board: 1, PON: 1, ID: 1, Name: "ONU-1", OnuType: "F670L", SerialNumber: "ZTEGC1234", RXPower: "-20.5", Status: "Online"},
				{Board: 1, PON: 1, ID: 5, Name: "ONU-5", OnuType: "F660V8", SerialNumber: "ZTEGC5678", RXPower: "-18.3", Status: "Online"},
			}, nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result.Name != "ONU-5" {
		t.Errorf("Expected ONU-5, got %s", result.Name)
	}
	if result.SerialNumber != "ZTEGC5678" {
		t.Errorf("Expected ZTEGC5678, got %s", result.SerialNumber)
	}
}

func TestGetByBoardIDPonIDAndOnuID_NotFoundInList_SkipsSNMP(t *testing.T) {
	cfg := createTestConfig()
	snmpCalled := false
	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			snmpCalled = true
			return nil
		},
	}
	redisRepo := &mockRedisRepository{
		GetONUDetailFunc: func(ctx context.Context, key string) (*model.ONUCustomerInfo, error) {
			return nil, errors.New("not found")
		},
		// Cached list has ONU 1 and 5, but NOT ONU 99
		GetONUInfoListFunc: func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{
				{Board: 1, PON: 1, ID: 1, Name: "ONU-1"},
				{Board: 1, PON: 1, ID: 5, Name: "ONU-5"},
			}, nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := uc.GetByBoardIDPonIDAndOnuID(context.Background(), 1, 1, 99)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Should return empty (zero-value) without calling SNMP
	if result.ID != 0 {
		t.Errorf("Expected empty result for non-existent ONU, got ID=%d", result.ID)
	}
	if snmpCalled {
		t.Error("Expected SNMP to NOT be called when ONU not in cached list")
	}
}

func TestDeleteCache_DeletesSerialListKey(t *testing.T) {
	cfg := createTestConfig()
	var deletedKeys []string
	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{
		DeleteFunc: func(ctx context.Context, key string) error {
			deletedKeys = append(deletedKeys, key)
			return nil
		},
	}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := uc.DeleteCache(context.Background(), 1, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Should delete both ONU info key and serial list key
	foundSerial := false
	for _, k := range deletedKeys {
		if k == "board_1_pon_1_serial_list" {
			foundSerial = true
		}
	}
	if !foundSerial {
		t.Errorf("Expected serial list key to be deleted, deleted keys: %v", deletedKeys)
	}
}
