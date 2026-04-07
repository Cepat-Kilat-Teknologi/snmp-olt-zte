# SNMP Performance Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce SNMP-related timeouts from 42% to <5% under 100 VU load by adding concurrency limiting, cache pre-warming, and configurable TTL.

**Architecture:** Three changes: (1) Add semaphore to SNMP repository to limit concurrent OLT operations, (2) Pre-warm Redis cache at startup by scanning all 32 board/pon combos, (3) Make Redis TTL configurable with higher defaults. All changes are backward compatible.

**Tech Stack:** Go stdlib `sync` for semaphore, existing Redis/SNMP infrastructure.

---

### Task 1: Add SNMP Semaphore to Repository

**Files:**
- Modify: `internal/repository/snmp.go`
- Modify: `internal/repository/snmp_test.go`

**Context:** The SNMP repository has a connection pool of 4, but there's no limit on how many goroutines can queue for a connection. Under load, 100 goroutines all queue for 4 connections, creating a thundering herd on the OLT device. A semaphore limits how many operations can be in-flight simultaneously.

**Step 1: Add semaphore to snmpRepository struct**

In `internal/repository/snmp.go`, add a `sem` field and import `sync`:

```go
import (
    "fmt"
    "sync"
    "time"

    "github.com/gosnmp/gosnmp"
)

type snmpRepository struct {
    pool chan *gosnmp.GoSNMP
    cfg  snmpConfig
    sem  chan struct{} // semaphore to limit concurrent SNMP operations
}
```

**Step 2: Initialize semaphore in NewPonRepository**

Add `MaxConcurrent` parameter. Update `NewPonRepository`:

```go
const DefaultPoolSize = 4
const DefaultMaxConcurrent = 5

func NewPonRepository(seed *gosnmp.GoSNMP) SnmpRepositoryInterface {
    return NewPonRepositoryWithConcurrency(seed, DefaultMaxConcurrent)
}

func NewPonRepositoryWithConcurrency(seed *gosnmp.GoSNMP, maxConcurrent int) SnmpRepositoryInterface {
    cfg := snmpConfig{
        target:    seed.Target,
        port:      seed.Port,
        community: seed.Community,
        version:   seed.Version,
        timeout:   seed.Timeout,
        retries:   seed.Retries,
        maxOids:   seed.MaxOids,
    }

    pool := make(chan *gosnmp.GoSNMP, DefaultPoolSize)
    pool <- seed
    for i := 1; i < DefaultPoolSize; i++ {
        conn, err := createConnection(cfg)
        if err != nil {
            break
        }
        pool <- conn
    }

    if maxConcurrent <= 0 {
        maxConcurrent = DefaultMaxConcurrent
    }

    return &snmpRepository{
        pool: pool,
        cfg:  cfg,
        sem:  make(chan struct{}, maxConcurrent),
    }
}
```

**Step 3: Wrap Get/Walk/BulkWalk with semaphore**

```go
func (r *snmpRepository) Get(oids []string) (*gosnmp.SnmpPacket, error) {
    r.sem <- struct{}{}
    defer func() { <-r.sem }()

    conn := r.acquire()
    defer r.release(conn)

    result, err := conn.Get(oids)
    if err != nil {
        return nil, fmt.Errorf("SNMP Get failed: %w", err)
    }
    return result, nil
}

func (r *snmpRepository) Walk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
    r.sem <- struct{}{}
    defer func() { <-r.sem }()

    conn := r.acquire()
    defer r.release(conn)

    err := conn.Walk(oid, walkFunc)
    if err != nil {
        return fmt.Errorf("SNMP Walk failed: %w", err)
    }
    return nil
}

func (r *snmpRepository) BulkWalk(oid string, walkFunc func(pdu gosnmp.SnmpPDU) error) error {
    r.sem <- struct{}{}
    defer func() { <-r.sem }()

    conn := r.acquire()
    defer r.release(conn)

    err := conn.BulkWalk(oid, walkFunc)
    if err != nil {
        return fmt.Errorf("SNMP BulkWalk failed: %w", err)
    }
    return nil
}
```

**Step 4: Add SNMP_MAX_CONCURRENT to config**

In `config/config.go`, add to `SnmpConfig`:
```go
type SnmpConfig struct {
    IP            string `mapstructure:"ip"`
    Port          uint16 `mapstructure:"port"`
    Community     string `mapstructure:"community"`
    MaxConcurrent int    `mapstructure:"max_concurrent"`
}
```

In `LoadConfig()`:
```go
cfg.SnmpCfg = SnmpConfig{
    IP:            getEnv("SNMP_HOST", ""),
    Port:          getEnvAsUint16("SNMP_PORT", 161),
    Community:     getEnv("SNMP_COMMUNITY", ""),
    MaxConcurrent: getEnvAsInt("SNMP_MAX_CONCURRENT", 5),
}
```

**Step 5: Wire in app.go**

In `app/app.go`, change:
```go
snmpRepo := repository.NewPonRepository(snmpConn)
```
to:
```go
snmpRepo := repository.NewPonRepositoryWithConcurrency(snmpConn, cfg.SnmpCfg.MaxConcurrent)
```

**Step 6: Run tests**

Run: `go test ./internal/repository/ ./config/ ./app/ -count=1`
Expected: All PASS.

---

### Task 2: Make Redis TTL Configurable

**Files:**
- Modify: `config/config.go` (add TTL fields to Config)
- Modify: `internal/usecase/onu.go` (use config TTL instead of constants)

**Step 1: Add TTL config fields**

In `config/config.go`, add a new struct and field:

```go
type CacheConfig struct {
    ONUInfoTTL    int // REDIS_ONU_INFO_TTL (seconds, default 1800)
    ONUDetailTTL  int // REDIS_ONU_DETAIL_TTL (seconds, default 900)
    EmptyOnuIDTTL int // REDIS_EMPTY_ONU_ID_TTL (seconds, default 300)
}
```

Add `CacheCfg CacheConfig` to the `Config` struct.

In `LoadConfig()`, add:
```go
cfg.CacheCfg = CacheConfig{
    ONUInfoTTL:    getEnvAsInt("REDIS_ONU_INFO_TTL", 1800),
    ONUDetailTTL:  getEnvAsInt("REDIS_ONU_DETAIL_TTL", 900),
    EmptyOnuIDTTL: getEnvAsInt("REDIS_EMPTY_ONU_ID_TTL", 300),
}
```

**Step 2: Use config TTL in usecase**

In `internal/usecase/onu.go`, replace hardcoded constants usage:
- `RedisONUInfoTTL` (600) → `u.cfg.CacheCfg.ONUInfoTTL` 
- `RedisONUDetailTTL` (120) → `u.cfg.CacheCfg.ONUDetailTTL`
- `RedisEmptyOnuIDTTL` (300) → `u.cfg.CacheCfg.EmptyOnuIDTTL`
- `RedisONUInfoRefreshThreshold` (120) → compute as 20% of `u.cfg.CacheCfg.ONUInfoTTL`

Keep the constants as fallback defaults but use config values in the actual code paths.

**Step 3: Run tests**

Run: `go test ./config/ ./internal/usecase/ -count=1`
Expected: All PASS.

---

### Task 3: Add Cache Pre-warming at Startup

**Files:**
- Create: `internal/usecase/prewarm.go`
- Create: `internal/usecase/prewarm_test.go`
- Modify: `app/app.go`
- Modify: `config/config.go`

**Step 1: Add config flag**

In `config/config.go`, add to `CacheConfig`:
```go
type CacheConfig struct {
    ONUInfoTTL    int
    ONUDetailTTL  int
    EmptyOnuIDTTL int
    PreWarm       bool // CACHE_PREWARM (default true)
}
```

In `LoadConfig()`:
```go
PreWarm: getEnv("CACHE_PREWARM", "true") == "true",
```

**Step 2: Create prewarm.go**

```go
package usecase

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
)

// PreWarmCache scans all board/pon combinations to populate Redis cache.
// This runs as a background goroutine at startup so first requests hit cache.
func (u *onuUsecase) PreWarmCache(ctx context.Context) {
    log.Info().Msg("Cache pre-warm: starting")
    start := time.Now()
    total := 0
    errors := 0

    for boardID := 1; boardID <= 2; boardID++ {
        for ponID := 1; ponID <= 16; ponID++ {
            select {
            case <-ctx.Done():
                log.Warn().Msg("Cache pre-warm: cancelled")
                return
            default:
            }

            _, err := u.GetByBoardIDAndPonID(ctx, boardID, ponID)
            if err != nil {
                errors++
                log.Debug().Err(err).Int("board", boardID).Int("pon", ponID).
                    Msg("Cache pre-warm: failed to fetch")
            } else {
                total++
            }
        }
    }

    log.Info().
        Int("success", total).
        Int("errors", errors).
        Dur("duration", time.Since(start)).
        Msg("Cache pre-warm: completed")
}
```

**Step 3: Add PreWarmCache to interface**

In `internal/usecase/onu.go`, add to `OnuUseCaseInterface`:
```go
PreWarmCache(ctx context.Context)
```

**Step 4: Write test**

In `internal/usecase/prewarm_test.go`:
```go
package usecase

import (
    "context"
    "testing"

    "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
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

    // Setup all 32 board/pon configs
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
    // Should not panic
    uc.PreWarmCache(context.Background())
}

func TestPreWarmCache_Cancelled(t *testing.T) {
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
    cancel() // Cancel immediately
    uc.PreWarmCache(ctx) // Should return quickly without panic
}
```

**Step 5: Wire in app.go**

In `app/app.go`, after `onuUsecase` is created (around line 88), add:

```go
// Pre-warm cache in background
if cfg.CacheCfg.PreWarm {
    go onuUsecase.PreWarmCache(ctx)
}
```

**Step 6: Run tests**

Run: `go test ./... -count=1`
Expected: All PASS.

---

### Task 4: Update .env.example

**Files:**
- Modify: `.env.example`

Add these sections:

After SNMP Configuration section, add `SNMP_MAX_CONCURRENT`:
```env
# Maximum concurrent SNMP operations to OLT device (prevents OLT saturation)
# Increase if OLT can handle more; decrease if seeing SNMP timeouts
SNMP_MAX_CONCURRENT=5
```

After Redis pool settings, add cache TTL section:
```env
# Cache TTL Configuration
# Higher TTL = less SNMP polling, more stale data
# Lower TTL = fresher data, more SNMP load
REDIS_ONU_INFO_TTL=1800
REDIS_ONU_DETAIL_TTL=900
REDIS_EMPTY_ONU_ID_TTL=300

# Pre-warm cache on startup (scans all 32 board/pon into Redis)
CACHE_PREWARM=true
```

---

### Task 5: Run k6 load test and verify improvement

**Step 1: Run full test suite**

Run: `go test ./... -coverprofile=coverage.out -count=1`
Expected: All PASS, coverage >= 99%.

**Step 2: Restart server and run k6**

User restarts server, then run:
```bash
k6 run scripts/k6-load-test.js
```

Expected improvements:
- Error rate: 42% → <5%
- p(95) latency: 30s → <5s
- Most requests served from cache after pre-warm
