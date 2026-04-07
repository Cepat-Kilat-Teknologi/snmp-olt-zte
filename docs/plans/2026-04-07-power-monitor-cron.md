# Power Monitor Cron Schedule Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add cron-based scheduling to Power Monitor alongside existing interval mode, with configurable timezone.

**Architecture:** Extend `PowerMonitor` to support dual scheduling: existing `time.Ticker` for interval mode AND `robfig/cron/v3` for cron mode. Both call the existing `safeScan()`. Config determines which modes are active based on `POWER_MONITOR_INTERVAL` (>0) and `POWER_MONITOR_CRON` (non-empty). A new `POWER_MONITOR_TIMEZONE` env var controls cron timezone.

**Tech Stack:** `github.com/robfig/cron/v3` for cron scheduling.

**Behavior Table:**

| `INTERVAL` | `CRON`  | Result         |
|------------|---------|----------------|
| `0`        | set     | Cron only      |
| `> 0`      | set     | Interval + Cron|
| `> 0`      | kosong  | Interval only  |
| `0`        | kosong  | Disabled       |

---

### Task 1: Add `robfig/cron/v3` dependency

**Step 1: Install the dependency**

Run: `go get github.com/robfig/cron/v3`
Expected: `go.mod` and `go.sum` updated.

**Step 2: Verify**

Run: `grep robfig go.mod`
Expected: `github.com/robfig/cron/v3 v3.x.x`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add robfig/cron/v3 dependency for power monitor scheduling"
```

---

### Task 2: Add cron config fields to TrapConfig

**Files:**
- Modify: `config/config.go:43-54` (TrapConfig struct)
- Modify: `config/config.go:169-181` (LoadConfig TrapCfg block)

**Step 1: Add fields to TrapConfig struct**

In `config/config.go:43-54`, add two new fields to `TrapConfig`:

```go
type TrapConfig struct {
	Enabled              bool
	Port                 uint16
	Community            string
	WebhookURL           string
	WebhookRetries       int
	WebhookTimeout       int
	PowerMonitor         bool
	PowerMonitorInterval int
	PowerMonitorCron     string  // POWER_MONITOR_CRON (cron expression, e.g. "0 8,12,15,17,0 * * *")
	PowerMonitorTimezone string  // POWER_MONITOR_TIMEZONE (IANA timezone, e.g. "Asia/Jakarta")
	RxPowerHighThreshold float64
	RxPowerLowThreshold  float64
}
```

**Step 2: Load new env vars in LoadConfig**

In `config/config.go:169-181`, add the two new fields to the TrapCfg initialization:

```go
PowerMonitorCron:     getEnv("POWER_MONITOR_CRON", ""),
PowerMonitorTimezone: getEnv("POWER_MONITOR_TIMEZONE", ""),
```

**Step 3: Run existing tests**

Run: `go test ./config/ -v -count=1`
Expected: All PASS (no breaking changes).

**Step 4: Commit**

```bash
git add config/config.go
git commit -m "feat: add POWER_MONITOR_CRON and POWER_MONITOR_TIMEZONE config fields"
```

---

### Task 3: Add cron fields to PowerMonitorConfig and PowerMonitor struct

**Files:**
- Modify: `internal/trap/power_monitor.go:18-33`

**Step 1: Add Cron and Timezone to PowerMonitorConfig**

```go
type PowerMonitorConfig struct {
	Interval      time.Duration
	Cron          string // cron expression (5-field), e.g. "0 8,12,15,17,0 * * *"
	Timezone      string // IANA timezone, e.g. "Asia/Jakarta" (empty = local)
	HighThreshold float64
	LowThreshold  float64
	Source        string
}
```

**Step 2: Add `cronScheduler` field to PowerMonitor struct**

```go
import "github.com/robfig/cron/v3"

type PowerMonitor struct {
	config     PowerMonitorConfig
	fetcher    ONUListFetcher
	webhook    *WebhookClient
	stopCh     chan struct{}
	alerted    map[string]time.Time
	cronRunner *cron.Cron // nil if cron not configured
}
```

**Step 3: Run existing tests**

Run: `go test ./internal/trap/ -v -count=1`
Expected: All PASS.

**Step 4: Commit**

```bash
git add internal/trap/power_monitor.go
git commit -m "feat: add cron and timezone fields to PowerMonitor structs"
```

---

### Task 4: Rewrite `Start()` and `Close()` for dual scheduling

**Files:**
- Modify: `internal/trap/power_monitor.go:46-72` (Start and Close methods)

**Step 1: Write tests for the new scheduling modes**

**File:** `internal/trap/power_monitor_test.go` — add these tests:

```go
func TestPowerMonitor_CronOnly(t *testing.T) {
	var scanCount atomic.Int32

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "ONU1", RXPower: "-15.00", Status: "Online"},
			},
		},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0, // no interval
		Cron:          "* * * * *", // every minute (for test we trigger manually)
		Timezone:      "",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Verify cron is configured
	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set when Cron is configured")
	}

	// Manual scan should work
	pm.safeScan()

	// Close should not panic
	err := pm.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
}

func TestPowerMonitor_IntervalAndCron(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      100 * time.Millisecond,
		Cron:          "* * * * *",
		Timezone:      "Asia/Jakarta",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Both should be configured
	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set")
	}

	go pm.Start()
	time.Sleep(300 * time.Millisecond)
	pm.Close()
}

func TestPowerMonitor_IntervalOnly_BackwardCompat(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      100 * time.Millisecond,
		Cron:          "", // no cron
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner != nil {
		t.Error("Expected cronRunner to be nil when no cron configured")
	}

	go pm.Start()
	time.Sleep(300 * time.Millisecond)
	pm.Close()
}

func TestPowerMonitor_BothDisabled(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Start should return immediately (nothing to do)
	done := make(chan struct{})
	go func() {
		pm.Start()
		close(done)
	}()

	select {
	case <-done:
		// OK - Start returned because both disabled
	case <-time.After(2 * time.Second):
		t.Error("Expected Start to return immediately when both disabled")
		pm.Close()
	}
}

func TestPowerMonitor_InvalidCron(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "invalid cron expression",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Invalid cron should be treated as no cron (cronRunner nil)
	if pm.cronRunner != nil {
		t.Error("Expected cronRunner to be nil for invalid cron expression")
	}
}

func TestPowerMonitor_InvalidTimezone(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "0 8 * * *",
		Timezone:      "Invalid/Timezone",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Invalid timezone should fallback to local, cron still works
	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set even with invalid timezone (fallback to local)")
	}

	pm.Close()
}

func TestPowerMonitor_CronWithTimezone(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "0 8,12,15,17,0 * * *",
		Timezone:      "Asia/Jakarta",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set")
	}

	pm.Close()
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/trap/ -run "TestPowerMonitor_Cron|TestPowerMonitor_IntervalAndCron|TestPowerMonitor_IntervalOnly_BackwardCompat|TestPowerMonitor_BothDisabled|TestPowerMonitor_Invalid" -v -count=1`
Expected: FAIL (cronRunner field does not exist yet, Start behavior unchanged).

**Step 3: Implement `NewPowerMonitor` with cron setup**

Replace `NewPowerMonitor` in `internal/trap/power_monitor.go:36-44`:

```go
func NewPowerMonitor(cfg PowerMonitorConfig, fetcher ONUListFetcher, webhook *WebhookClient) *PowerMonitor {
	pm := &PowerMonitor{
		config:  cfg,
		fetcher: fetcher,
		webhook: webhook,
		stopCh:  make(chan struct{}),
		alerted: make(map[string]time.Time),
	}

	// Setup cron scheduler if cron expression is provided
	if cfg.Cron != "" {
		var opts []cron.Option

		// Parse timezone
		if cfg.Timezone != "" {
			loc, err := time.LoadLocation(cfg.Timezone)
			if err != nil {
				log.Warn().Str("timezone", cfg.Timezone).Err(err).
					Msg("Invalid timezone, falling back to local")
			} else {
				opts = append(opts, cron.WithLocation(loc))
			}
		}

		cronRunner := cron.New(opts...)
		_, err := cronRunner.AddFunc(cfg.Cron, pm.safeScan)
		if err != nil {
			log.Error().Str("cron", cfg.Cron).Err(err).
				Msg("Invalid cron expression, cron scheduling disabled")
		} else {
			pm.cronRunner = cronRunner
		}
	}

	return pm
}
```

**Step 4: Implement new `Start()` method**

Replace `Start()` in `internal/trap/power_monitor.go:47-66`:

```go
func (pm *PowerMonitor) Start() {
	hasInterval := pm.config.Interval > 0
	hasCron := pm.cronRunner != nil

	if !hasInterval && !hasCron {
		log.Warn().Msg("Power monitor: both interval and cron are disabled, nothing to do")
		return
	}

	log.Info().
		Dur("interval", pm.config.Interval).
		Str("cron", pm.config.Cron).
		Str("timezone", pm.config.Timezone).
		Float64("high_threshold", pm.config.HighThreshold).
		Float64("low_threshold", pm.config.LowThreshold).
		Msg("Starting RX Power monitor")

	// Start cron scheduler if configured
	if hasCron {
		pm.cronRunner.Start()
		log.Info().Str("cron", pm.config.Cron).Msg("Power monitor: cron scheduler started")
	}

	// Run interval ticker if configured
	if hasInterval {
		ticker := time.NewTicker(pm.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.safeScan()
			case <-pm.stopCh:
				log.Info().Msg("Power monitor stopped")
				return
			}
		}
	} else {
		// Cron-only mode: block until stop signal
		<-pm.stopCh
		log.Info().Msg("Power monitor stopped")
	}
}
```

**Step 5: Implement new `Close()` method**

Replace `Close()` in `internal/trap/power_monitor.go:68-72`:

```go
func (pm *PowerMonitor) Close() error {
	if pm.cronRunner != nil {
		pm.cronRunner.Stop()
	}
	close(pm.stopCh)
	return nil
}
```

**Step 6: Run tests**

Run: `go test ./internal/trap/ -v -count=1`
Expected: All PASS.

**Step 7: Commit**

```bash
git add internal/trap/power_monitor.go internal/trap/power_monitor_test.go
git commit -m "feat: add cron scheduling support to power monitor"
```

---

### Task 5: Wire cron config in app.go

**Files:**
- Modify: `app/app.go:122-137`

**Step 1: Pass Cron and Timezone to PowerMonitorConfig**

In `app/app.go:122-137`, update the `PowerMonitor` check and initialization:

```go
		// Start RX Power monitor if enabled
		if cfg.TrapCfg.PowerMonitor && webhookClient != nil {
			powerMonitor := trap.NewPowerMonitor(trap.PowerMonitorConfig{
				Interval:      time.Duration(cfg.TrapCfg.PowerMonitorInterval) * time.Second,
				Cron:          cfg.TrapCfg.PowerMonitorCron,
				Timezone:      cfg.TrapCfg.PowerMonitorTimezone,
				HighThreshold: cfg.TrapCfg.RxPowerHighThreshold,
				LowThreshold:  cfg.TrapCfg.RxPowerLowThreshold,
				Source:        cfg.SnmpCfg.IP,
			}, onuUsecase, webhookClient)
			go powerMonitor.Start()
			defer powerMonitor.Close()
			log.Info().
				Float64("high_threshold", cfg.TrapCfg.RxPowerHighThreshold).
				Float64("low_threshold", cfg.TrapCfg.RxPowerLowThreshold).
				Int("interval_sec", cfg.TrapCfg.PowerMonitorInterval).
				Str("cron", cfg.TrapCfg.PowerMonitorCron).
				Str("timezone", cfg.TrapCfg.PowerMonitorTimezone).
				Msg("RX Power monitor started")
		}
```

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: All PASS.

**Step 3: Commit**

```bash
git add app/app.go
git commit -m "feat: wire cron config to power monitor in app startup"
```

---

### Task 6: Update .env.example

**Files:**
- Modify: `.env.example` (RX Power Monitor section)

**Step 1: Update the Power Monitor section**

Replace the RX Power Monitor section with:

```env
# ------------------------------------------------------------------------------
# RX Power Monitor
# ------------------------------------------------------------------------------
# Periodic scanning of all ONUs for abnormal optical power levels.
# Sends webhook alerts when RX power exceeds thresholds (overload or weak signal).
# Requires: TRAP_ENABLED=true AND TRAP_WEBHOOK_URL to be set.
#
# Scheduling modes (can combine both):
#   INTERVAL only:  POWER_MONITOR_INTERVAL=300, POWER_MONITOR_CRON=
#   CRON only:      POWER_MONITOR_INTERVAL=0,   POWER_MONITOR_CRON=0 8,12,15,17,0 * * *
#   Both:           POWER_MONITOR_INTERVAL=300,  POWER_MONITOR_CRON=0 8,12,15,17,0 * * *
#   Disabled:       POWER_MONITOR_INTERVAL=0,    POWER_MONITOR_CRON=
#
# POWER_MONITOR_ENABLED: Enable/disable the RX power scanner (true/false)
# POWER_MONITOR_INTERVAL: Scan interval in seconds (0 = disable interval mode)
# POWER_MONITOR_CRON: Cron expression for scheduled scans (5-field standard cron)
#   Examples:
#     0 8,12,15,17,0 * * *    = at 08:00, 12:00, 15:00, 17:00, 00:00
#     */30 * * * *             = every 30 minutes
#     0 */2 * * *              = every 2 hours
#     0 6 * * 1-5              = weekdays at 06:00
# POWER_MONITOR_TIMEZONE: IANA timezone for cron schedule (empty = server local time)
#   Examples: Asia/Jakarta, Asia/Makassar, UTC, America/New_York
# RX_POWER_HIGH_THRESHOLD: Alert when RX power exceeds this value in dBm (overload risk)
# RX_POWER_LOW_THRESHOLD: Alert when RX power drops below this value in dBm (weak signal)
POWER_MONITOR_ENABLED=false
POWER_MONITOR_INTERVAL=300
POWER_MONITOR_CRON=
POWER_MONITOR_TIMEZONE=Asia/Jakarta
RX_POWER_HIGH_THRESHOLD=-8.0
RX_POWER_LOW_THRESHOLD=-25.0
```

**Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: update .env.example with cron scheduling options"
```

---

### Task 7: Run full test suite and verify coverage

**Step 1: Run full tests with coverage**

Run: `go test ./... -coverprofile=coverage.out -count=1`
Expected: All PASS, coverage >= 99%.

**Step 2: Check coverage of new code**

Run: `go tool cover -func=coverage.out | grep power_monitor`
Expected: `power_monitor.go` coverage >= 95%.

**Step 3: Final commit if any fixes needed**

```bash
git add -A
git commit -m "test: ensure full coverage for power monitor cron feature"
```
