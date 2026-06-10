package usecase

import (
	"context"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/gosnmp/gosnmp"
)

func TestPreWarmCache_Success(t *testing.T) {
	cfg := &config.Config{
		OltCfg: config.OltConfig{
			BaseOID1: "1.3.6.1.4.1",
			BaseOID2: "1.3.6.1.4.2",
		},
		CacheCfg: config.CacheConfig{
			ONUInfoTTL:    1800,
			ONUDetailTTL:  900,
			EmptyOnuIDTTL: 300,
		},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}

	for b := 1; b <= 2; b++ {
		for p := 1; p <= 16; p++ {
			cfg.BoardPonMap[config.BoardPonKey{BoardID: b, PonID: p}] = &config.BoardPonConfig{
				OnuIDNameOID: ".1.1.1",
			}
		}
	}

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("TestONU"),
			})
		},
	}
	redisRepo := &mockRedisRepository{}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	uc.PreWarmCache(context.Background())
}

func TestPreWarmCache_Canceled(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{BaseOID1: "1.3.6.1.4.1", BaseOID2: "1.3.6.1.4.2"},
		CacheCfg:    config.CacheConfig{ONUInfoTTL: 1800, ONUDetailTTL: 900, EmptyOnuIDTTL: 300},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	for b := 1; b <= 2; b++ {
		for p := 1; p <= 16; p++ {
			cfg.BoardPonMap[config.BoardPonKey{BoardID: b, PonID: p}] = &config.BoardPonConfig{
				OnuIDNameOID: ".1.1.1",
			}
		}
	}

	snmpRepo := &mockSnmpRepository{}
	redisRepo := &mockRedisRepository{}
	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	uc.PreWarmCache(ctx)
}

func TestPreWarmCache_WithErrors(t *testing.T) {
	cfg := &config.Config{
		OltCfg:      config.OltConfig{BaseOID1: "1.3.6.1.4.1", BaseOID2: "1.3.6.1.4.2"},
		CacheCfg:    config.CacheConfig{ONUInfoTTL: 1800, ONUDetailTTL: 900, EmptyOnuIDTTL: 300},
		BoardPonMap: make(map[config.BoardPonKey]*config.BoardPonConfig),
	}
	// Only add board 1 pon 1 — all other combos will fail with "config not found"
	cfg.BoardPonMap[config.BoardPonKey{BoardID: 1, PonID: 1}] = &config.BoardPonConfig{
		OnuIDNameOID: ".1.1.1",
	}

	snmpRepo := &mockSnmpRepository{
		BulkWalkFunc: func(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
			return walkFunc(gosnmp.SnmpPDU{
				Name: oid + ".1", Type: gosnmp.OctetString, Value: []byte("TestONU"),
			})
		},
	}
	redisRepo := &mockRedisRepository{}

	uc := NewOnuUsecase(snmpRepo, redisRepo, cfg)
	// Should not panic, should log errors for missing configs
	uc.PreWarmCache(context.Background())
}
