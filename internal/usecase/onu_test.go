package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/gosnmp/gosnmp"
)

// mockSnmpRepository is a mock implementation of SnmpRepositoryInterface
type mockSnmpRepository struct {
	GetFunc  func(oids []string) (*gosnmp.SnmpPacket, error)
	WalkFunc func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error
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

// mockRedisRepository is a mock implementation of OnuRedisRepositoryInterface
type mockRedisRepository struct {
	GetONUInfoListFunc  func(ctx context.Context, key string) ([]model.ONUInfoPerBoard, error)
	SaveONUInfoListFunc func(ctx context.Context, key string, seconds int, onuInfoList []model.ONUInfoPerBoard) error
	GetOnuIDCtxFunc     func(ctx context.Context, key string) ([]model.OnuID, error)
	SetOnuIDCtxFunc     func(ctx context.Context, key string, seconds int, onuID []model.OnuID) error
	DeleteFunc          func(ctx context.Context, key string) error
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

func (m *mockRedisRepository) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
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
	var usecase OnuUseCaseInterface = NewOnuUsecase(
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

	redisRepo := &mockRedisRepository{}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	result, err := usecase.GetByBoardIDAndPonID(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
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
	result, err := usecase.GetByBoardIDPonIDAndOnuID(1, 1, 5)

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
	result, err := usecase.GetOnuIDAndSerialNumber(1, 1)

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
	result, count := usecase.GetByBoardIDAndPonIDWithPagination(1, 1, 1, 5)

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
	_, err := usecase.GetOnuIDAndSerialNumber(1, 1)

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
	_, err := usecase.GetByBoardIDPonIDAndOnuID(1, 1, 5)

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

func TestConstants_RedisONUInfoTTL(t *testing.T) {
	// RedisONUInfoTTL should be 600 seconds (10 minutes)
	if RedisONUInfoTTL != 600 {
		t.Errorf("Expected RedisONUInfoTTL to be 600, got %d", RedisONUInfoTTL)
	}
}

func TestConstants_RedisEmptyOnuIDTTL(t *testing.T) {
	// RedisEmptyOnuIDTTL should be 300 seconds (5 minutes)
	if RedisEmptyOnuIDTTL != 300 {
		t.Errorf("Expected RedisEmptyOnuIDTTL to be 300, got %d", RedisEmptyOnuIDTTL)
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

	// Verify the TTL used matches RedisEmptyOnuIDTTL constant
	if capturedTTL != RedisEmptyOnuIDTTL {
		t.Errorf("Expected TTL to be %d (RedisEmptyOnuIDTTL), got %d", RedisEmptyOnuIDTTL, capturedTTL)
	}
}

func TestGetByBoardIDAndPonID_UsesRedisONUInfoTTL(t *testing.T) {
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

	// Verify the TTL used matches RedisONUInfoTTL constant
	if capturedTTL != RedisONUInfoTTL {
		t.Errorf("Expected TTL to be %d (RedisONUInfoTTL), got %d", RedisONUInfoTTL, capturedTTL)
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

	var capturedKey string
	snmpRepo := &mockSnmpRepository{}

	redisRepo := &mockRedisRepository{
		DeleteFunc: func(ctx context.Context, key string) error {
			capturedKey = key
			return nil
		},
	}

	usecase := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	err := usecase.DeleteCache(context.Background(), 1, 1)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the key used matches GenerateRedisKey output
	expectedKey := GenerateRedisKey(RedisKeyTypeONUInfo, 1, 1)
	if capturedKey != expectedKey {
		t.Errorf("Expected Redis key to be %s, got %s", expectedKey, capturedKey)
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
