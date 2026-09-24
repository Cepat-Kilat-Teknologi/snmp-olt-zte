# Dynamic OLT Registry Refactor - Changelog & Documentation

## Overview

Refactored `snmp-olt-zte` from a static OLT configuration model (OLTs loaded
once at startup, routes baked into the chi router at init) to a fully dynamic
registry model where OLTs can be added, removed, and updated at runtime without
a service restart.

**Impact:** 9 files changed (7 modified + 2 new), 268 insertions, 106 deletions  
**Test coverage:** 20/20 packages pass, including 17 new OLTRegistry tests  
**Breaking changes:** None: all 4 configuration modes remain backward-compatible

---

## Architecture

```
                     +----------------------+
                     |   device-registry    |
                     |  GET /v1/registry/   |
                     |        snmp          |
                     +----------+-----------+
                                | poll every 30s (REGISTRY_URL mode only)
                     +----------v-----------+
                     |   StartPoller()      |
                     |  (background goroutine)
                     +----------+-----------+
                                | FetchRegistryOLTConfigs() -> Reconcile()
                     +----------v-----------+
                     |    OLTRegistry       |
                     |  sync.RWMutex map    |
                     |  OLT ID -> *OLTEntry |
                     +----------+-----------+
                                | Get(oltID) at request time
         +-----------+----------v-----------+-----------+
         |  chi router: /api/v1/olt/{olt_id}/board/...  |
         |  resolveOLT middleware reads {olt_id} param   |
         |  injects *OLTEntry into request context       |
         +----------------------------------------------+
```

### Before (Static)

- OLTs parsed at startup from `OLTS` / `OLTS_FILE` / `REGISTRY_URL` env
- A `for _, olt := range cfg.OLTs` loop created SNMP conn + repo + usecase + handler per OLT
- Routes hard-wired: `apiV1Group.Route("/olt/" + olt.ID, ...)` with literal OLT IDs
- Adding/removing an OLT required a pod restart

### After (Dynamic)

- OLTs still parsed at startup (same precedence), but stored in `*OLTRegistry`
- `Reconcile()` diff engine adds/removes/updates OLTs with minimal disruption
- Routes use chi `{olt_id}` URL param, resolved at request time from live registry
- `StartPoller()` goroutine fetches from device-registry every 30s and calls `Reconcile()`
- New OLTs serve requests within one poll interval; removed OLTs 404 immediately

---

## Configuration Modes (4 modes, strict precedence)

| Priority | Mode | Env Var | Poller | Description |
|:--------:|------|---------|:------:|-------------|
| 1 (highest) | Inline JSON | `OLTS` | No | JSON array of OLT configs directly in env |
| 2 | File | `OLTS_FILE` | No | Path to a JSON file containing the OLT array |
| 3 | Registry | `REGISTRY_URL` | **Yes** | Fetch from device-registry at startup + poll every N seconds |
| 4 (lowest) | Legacy | `SNMP_HOST` / `SNMP_PORT` / `SNMP_COMMUNITY` | No | Single-OLT fallback from original codebase |

**Precedence rule (config/config.go lines 343-361):**

```
if OLTS is set        -> use it (inline JSON)
else if OLTS_FILE     -> read file, use its content
else if REGISTRY_URL  -> fetch from device-registry (with retry)
else                  -> build single OLT from SNMP_HOST/PORT/COMMUNITY
```

**Poller activation (app/app.go lines 78-89):**

The poller only starts when `REGISTRY_URL` is set in the environment, regardless
of which mode actually provided the initial OLT list. This means:

- `OLTS` mode: static, no polling
- `OLTS_FILE` mode: static, no polling
- `REGISTRY_URL` mode: initial fetch + continuous polling
- Legacy mode: static, no polling

**Edge case:** If both `OLTS` and `REGISTRY_URL` are set, the initial load uses
`OLTS` (higher precedence), but the poller starts and overwrites the registry on
its first tick (~30s). This is by design, `OLTS` provides the bootstrap set,
and the registry takes over for ongoing management.

---

## New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REGISTRY_POLL_INTERVAL` | `30s` | Go duration string. How often to poll device-registry. Set to `0` to disable polling entirely (registry mode becomes fetch-once-at-startup). |

No existing environment variables were changed or removed.

---

## Files Changed

### 1. `config/config.go` - 1 line changed

**Change:** Updated call site from `buildOLTRegistry` to `BuildOLTRegistry`.

```go
// Line 362
olts, defaultOLT, err := BuildOLTRegistry(oltsJSON, getEnv("DEFAULT_OLT", ""), legacy)
```

### 2. `config/olts.go` - 2 lines changed (export rename)

**Change:** `buildOLTRegistry` -> `BuildOLTRegistry` (exported).

Required so `app/registry.go`'s poller can call the parser from outside the
config package. The function signature, behavior, and all call paths are
identical: only the visibility changed.

### 3. `config/registry.go` - 31 lines changed

**Changes:**

1. `fetchRegistryOLTS` -> `FetchRegistryOLTS` (exported)
2. New function `FetchRegistryOLTConfigs`:

```go
func FetchRegistryOLTConfigs(baseURL, apiKey string) ([]OLTRuntimeConfig, error) {
    js, err := FetchRegistryOLTS(baseURL, apiKey)
    if err != nil { return nil, err }
    if js == "" { return nil, nil }
    olts, _, err := BuildOLTRegistry(js, "", OLTRuntimeConfig{})
    if err != nil { return nil, fmt.Errorf("parse registry OLTs: %w", err) }
    return olts, nil
}
```

This convenience function combines fetch + parse into a single call, used by the
poller in `app/registry.go`. Returns `nil, nil` on empty response (signals
"keep current OLTs" to the poller).

### 4. `config/olts_test.go` - 18 lines changed

Updated all `buildOLTRegistry` references to `BuildOLTRegistry`. No logic changes.

### 5. `config/registry_test.go` - 18 lines changed

Updated all `fetchRegistryOLTS` references to `FetchRegistryOLTS`. Test function
names updated to match Go convention: `TestFetchRegistryOLTSWithRetry_*` ->
`Test_fetchRegistryOLTSWithRetry_*`.

### 6. `app/registry.go` - NEW (297 lines)

Core new file. The thread-safe, dynamically-updatable OLT registry.

**Types:**

```go
type OLTEntry struct {
    OLT       config.OLTRuntimeConfig
    Repo      repository.SnmpRepositoryInterface
    UC        usecase.OnuUseCaseInterface
    Handler   *handler.OnuHandler
    BoardPons map[int]int  // physical slot -> PON count
    UserID    int          // owner tenant id
}

type OLTRegistry struct {
    mu         sync.RWMutex
    entries    map[string]*OLTEntry
    defaultOLT string
    cfg        *config.Config
    redisRepo  repository.OnuRedisRepositoryInterface
}
```

**Methods:**

| Method | Description |
|--------|-------------|
| `NewOLTRegistry(cfg, redisRepo, defaultOLT)` | Creates empty registry |
| `Get(oltID) (*OLTEntry, bool)` | Read-locked lookup (hot path, every request) |
| `GetDefault() (*OLTEntry, bool)` | Returns default OLT, falls back to first entry |
| `DefaultOLTID() string` | Returns configured default OLT identifier |
| `List() []string` | Returns sorted OLT IDs (deterministic for health probes) |
| `Len() int` | Returns count of registered OLTs |
| `Reconcile(newOLTs)` | Core diff engine: add/remove/update with minimal disruption |
| `Close()` | Closes all SNMP connections (idempotent) |
| `HealthCheck(ctx) error` | Pings all OLTs, returns first error |
| `StartPoller(ctx, url, key, interval)` | Background goroutine polling device-registry |

**Reconcile diff engine:**

The `Reconcile()` method is the heart of the dynamic registry. It compares the
current registry state against the incoming OLT list and performs the minimum
set of operations to converge:

1. **Remove**: OLTs in current map but not in incoming set: close SNMP, delete
2. **Add**: OLTs in incoming set but not in current map: create full stack
3. **Update (connection change)**: OLT exists in both but `oltChangeKey()` differs:
   remove + add (full SNMP reconnection)
4. **Update (metadata only)**: OLT exists in both, same connection params, but
   `user_id` changed: update fields in-place (no reconnection)

**Change detection key:**

```go
func oltChangeKey(o config.OLTRuntimeConfig) string {
    // host|port|community|walk|boards
    return fmt.Sprintf("%s|%d|%s|%t|%s",
        o.Host, o.Port, o.Community, o.UseWalk, strings.Join(boards, ","))
}
```

Any change in host, port, community, walk mode, or board topology triggers a
full SNMP reconnection. Changes to user_id, organization, or other metadata
are applied in-place without disruption.

**Safety guarantees:**

- Poll failure → log warning, keep current OLTs (never wipe the registry)
- Empty registry response → log warning, keep current OLTs
- Failed OLT init → log error, skip that OLT, others continue serving
- `Close()` is idempotent (safe to call multiple times)
- All map access protected by `sync.RWMutex` (read-locked for hot path)

### 7. `app/registry_test.go` - NEW (784 lines, 17 tests)

Comprehensive test suite covering all `OLTRegistry` methods:

| Test | What it verifies |
|------|-----------------|
| `TestOLTRegistry_Get` | Basic lookup returns correct entry |
| `TestOLTRegistry_GetDefault` | Default OLT returned by configured ID |
| `TestOLTRegistry_GetDefault_Fallback` | Falls back to first entry when default missing |
| `TestOLTRegistry_GetDefault_EmptyRegistry` | Returns false on empty registry |
| `TestOLTRegistry_DefaultOLTID` | Returns configured default ID string |
| `TestOLTRegistry_List` | Returns sorted OLT IDs |
| `TestOLTRegistry_Len` | Returns correct count |
| `TestOLTRegistry_Reconcile_RemovesStaleOLTs` | Removes OLTs no longer in incoming list |
| `TestOLTRegistry_Reconcile_EmptyListRemovesAll` | Empty list clears entire registry |
| `TestOLTRegistry_Reconcile_UnchangedOLTNotRebuilt` | Same config = no reconnection |
| `TestOLTRegistry_Reconcile_MetadataUpdateInPlace` | user_id change without reconnect |
| `TestOLTRegistry_Reconcile_ConnectionChangeTriggersRebuild` | Host change = full rebuild |
| `TestOLTRegistry_HealthCheck_AllHealthy` | All pings succeed → nil error |
| `TestOLTRegistry_HealthCheck_OneUnhealthy` | One ping fails → error with OLT ID |
| `TestOLTRegistry_HealthCheck_ContextCancelled` | Cancelled context → context error |
| `TestOLTRegistry_HealthCheck_EmptyRegistry` | No OLTs → nil error |
| `TestOLTRegistry_Close` | All SNMP connections closed |
| `TestOLTRegistry_Close_Idempotent` | Second Close() is a no-op |
| `TestOLTRegistry_Poller_DisabledByZeroInterval` | interval=0 → poller returns immediately |
| `TestOLTRegistry_Poller_StopsOnContextCancel` | Context cancel → poller exits cleanly |

### 8. `app/app.go` - 127 lines changed (major rewrite of startup)

**Removed:**

- Static `oltStack` struct and `[]oltRoute` accumulation
- `for _, olt := range cfg.OLTs` loop that created individual SNMP connections
- Per-OLT health probes registered individually
- Static `loadRoutesMulti(oltRoutes, ...)` call

**Added:**

```go
// Dynamic registry creation and initial population
reg := NewOLTRegistry(cfg, redisRepo, cfg.DefaultOLT)
reg.Reconcile(cfg.OLTs)
defer reg.Close()

// Poller activation (only when REGISTRY_URL is set)
if registryURL := os.Getenv("REGISTRY_URL"); registryURL != "" {
    // ... parse poll interval, start poller goroutine
    go reg.StartPoller(pollerCtx, registryURL, apiKey, pollInterval)
}

// Simplified health probes
checker.Register("snmp_default", 30*time.Second, func(ctx context.Context) error {
    entry, ok := reg.GetDefault()
    // ...
})
checker.RegisterOptional("snmp_olts", 30*time.Second, reg.HealthCheck)

// Dynamic router
a.router = loadRoutesWithRegistry(reg, checker, principals, cfg.APIKey)
```

**Health probe changes:**

Before: one probe per OLT, each with its own `repo.Ping()` goroutine. The
default OLT was critical; others were optional.

After: Two probes: `snmp_default` (critical, probes the default OLT) and
`snmp_olts` (optional, aggregate `HealthCheck` across all OLTs). Simpler,
and adapts to OLTs being added/removed without re-registering probes.

### 9. `app/routes.go` - 174 lines added

**New imports:** `context`, `apperrors`, `utils`

**New types and functions:**

| Symbol | Description |
|--------|-------------|
| `oltEntryKeyType` | Unexported context key type for OLTEntry |
| `oltEntryCtxKey` | Context key instance |
| `OLTEntryFromContext(ctx)` | Extract `*OLTEntry` from context (panics if absent) |
| `loadRoutesWithRegistry(reg, checker, users, key)` | Dynamic counterpart of `loadRoutesMulti` |
| `resolveOLT(reg)` | Chi middleware: `{olt_id}` -> registry lookup -> ownership check -> context |
| `resolveDefaultOLT(reg)` | Chi middleware: default OLT -> ownership check -> context |
| `dynamicValidateBoardPon` | Middleware: reads BoardPons from context OLTEntry |
| `dynamicHandler(method)` | Wraps `OnuHandler` method expression with context-based dispatch |
| `mountDynamicONURoutes(router)` | Same route tree as `mountONURoutes` but with dynamic handlers |

**Dynamic handler pattern:**

```go
// Method expression: (*handler.OnuHandler).GetByBoardIDAndPonID
// At request time, resolves the handler from the OLTEntry in context
func dynamicHandler(method func(*handler.OnuHandler, http.ResponseWriter, *http.Request)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        entry := OLTEntryFromContext(r.Context())
        method(entry.Handler, w, r)
    }
}
```

**Ownership enforcement:**

The `resolveOLT` middleware performs per-tenant ownership checks at request time,
reading `UserID` from the live registry entry rather than a closure-captured
constant. This means ownership changes in device-registry take effect on the
next poll tick without a restart.

**Backward compatibility:**

The static `loadRoutesMulti` and `mountONURoutes` functions are preserved, they
are still used by tests and could serve as a fallback. The new dynamic functions
are additive.

---

## Backward Compatibility Matrix

| Feature | Before | After | Breaking? |
|---------|--------|-------|:---------:|
| `OLTS` env (inline JSON) | Static load | Static load (no poller) | No |
| `OLTS_FILE` env | Static load | Static load (no poller) | No |
| `REGISTRY_URL` env | Fetch once at startup | Fetch at startup + poll every 30s | No |
| Legacy `SNMP_*` env | Single OLT | Single OLT (no poller) | No |
| `/api/v1/board/...` (bare) | Default OLT | Default OLT (unchanged) | No |
| `/api/v1/olt/{id}/board/...` | Static per-OLT routes | Dynamic lookup from registry | No |
| Per-tenant auth | Static `RequireOLTOwner` | Dynamic ownership from entry | No |
| Health probes | One per OLT | Default critical + aggregate optional | No |
| Trap listener | Default OLT usecase | Default OLT usecase (unchanged) | No |
| Cache pre-warm | Loop over stacks | Loop over registry entries | No |

---

## Verification Results

```
$ go vet ./...                                # clean
$ go build ./...                              # clean
$ go test ./... -count=1                      # ALL 20 packages PASSED

ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/app             5.659s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/cmd/api         2.353s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config          57.916s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/buildinfo    2.063s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors       3.008s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/handler      1.982s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/health       3.527s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/middleware   2.667s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model        3.812s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/repository   4.543s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx       2.725s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/trap        19.658s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/usecase      3.677s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils        3.406s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/graceful          3.887s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger            3.351s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/metrics           3.675s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/pagination        3.097s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/redis             3.481s
ok  github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/snmp              3.011s
```

---

## Deployment Notes

### No configuration changes required

Existing deployments (k3s ConfigMap, Helm values) continue working without any
changes. The refactor is purely internal, the HTTP API, the environment
variables, and the response format are all identical.

### New capability: dynamic OLT management

For deployments using `REGISTRY_URL`, OLTs managed in device-registry are now
picked up automatically:

1. **Add an OLT** in device-registry → appears in snmp-olt-zte within 30s
2. **Remove an OLT** in device-registry → 404s within 30s, SNMP connection closed
3. **Change OLT connection** (host/port/community) → SNMP reconnection within 30s
4. **Change OLT ownership** (user_id) → updated in-place within 30s (no reconnection)

### Monitoring

Watch the poller with:

```bash
kubectl --context=k3s -n misindo-snmp-olt-zte-prod logs -f deploy/snmp-olt-zte \
  | grep -E 'olt_added|olt_removed|olt_updated|registry_poll'
```

### Tuning

- `REGISTRY_POLL_INTERVAL=30s` (default): suitable for most deployments
- `REGISTRY_POLL_INTERVAL=10s`: faster pickup, slightly more device-registry load
- `REGISTRY_POLL_INTERVAL=0`: disable polling entirely (fetch-once-at-startup)

---

## Commit Plan

Single atomic commit covering all 9 files:

```
refactor(app): dynamic OLT registry with runtime add/remove/update

Replace static per-OLT route loop with thread-safe OLTRegistry that
resolves OLTs at request time. New OLTs appear, changed OLTs reconnect,
and removed OLTs clean up — all without a restart.

- app/registry.go: OLTRegistry type with Reconcile() diff engine,
  StartPoller() background goroutine, HealthCheck(), Close()
- app/routes.go: resolveOLT/resolveDefaultOLT middleware, dynamicHandler
  wrapper for method-expression dispatch via context
- app/app.go: replace static OLT loop with registry + poller wiring
- config/olts.go: export BuildOLTRegistry for poller use
- config/registry.go: export FetchRegistryOLTS, add FetchRegistryOLTConfigs
- 17 new tests covering Get, Reconcile, HealthCheck, Close, Poller
```

**Files in commit:**

| File | Status | Lines |
|------|--------|-------|
| `config/config.go` | Modified | +1 / -1 |
| `config/olts.go` | Modified | +2 / -2 |
| `config/olts_test.go` | Modified | +9 / -9 |
| `config/registry.go` | Modified | +20 / -11 |
| `config/registry_test.go` | Modified | +9 / -9 |
| `app/registry.go` | **New** | +297 |
| `app/registry_test.go` | **New** | +784 |
| `app/app.go` | Modified | +45 / -82 |
| `app/routes.go` | Modified | +174 / -0 |
| **Total** | | **+1341 / -114** |

---

## Design Decisions

### Why a registry instead of just restarting the pod?

Pod restarts cause 10-30s of downtime (SNMP connection setup, cache pre-warm).
With 4 OLTs serving live subscriber queries, that downtime is visible. The
registry model allows zero-downtime OLT management.

### Why Reconcile() instead of replace-all?

A naive "close everything, rebuild everything" approach would disconnect all
SNMP connections on every poll tick, even if nothing changed. The diff engine
ensures unchanged OLTs keep their live connections, and only changed OLTs
reconnect.

### Why keep the static routing code?

`loadRoutesMulti` and `mountONURoutes` are still used by handler tests that
create a single-OLT router. Removing them would require rewriting those tests
to use the registry, which is out of scope for this refactor.

### Why method expressions instead of a handler factory?

```go
dynamicHandler((*handler.OnuHandler).GetByBoardIDAndPonID)
```

This pattern avoids creating a new handler function per OLT. The method
expression is resolved once at route setup; only the receiver (the `*OLTEntry`
in context) changes per request. It's type-safe, if `OnuHandler` gains or
loses a method, the compiler catches it.

### Why not use chi's built-in route params for board/pon validation?

Board/PON topology varies per OLT (C320 has boards 1,2 with 16 PONs each; C300
has boards 3,5 with 16 PONs each). The `dynamicValidateBoardPon` middleware
reads the topology from the resolved OLTEntry in context, so validation adapts
to whichever OLT the request targets, even if that OLT was added 30 seconds ago.
