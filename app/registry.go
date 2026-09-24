package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/snmp"
	"go.uber.org/zap"
)

// defaultPollInterval is the default device-registry poll interval. Override
// with REGISTRY_POLL_INTERVAL (Go duration); "0" disables polling entirely.
const defaultPollInterval = 30 * time.Second

// OLTEntry holds the full runtime stack for a single OLT: SNMP connection pool,
// repository, usecase, handler, and the topology metadata the router needs for
// board/pon validation and per-tenant auth. Exported so routes.go can read its
// fields through the registry.
type OLTEntry struct {
	OLT       config.OLTRuntimeConfig
	Repo      repository.SnmpRepositoryInterface
	UC        usecase.OnuUseCaseInterface
	Handler   *handler.OnuHandler
	BoardPons map[int]int // physical slot -> PON count (used by ValidateBoardPonParams)
	UserID    int         // owner tenant id (used by RequireOLTOwner)

	// Lifecycle of background work bound to this entry (see GoForEach). It is
	// shared by the copies Reconcile makes on a metadata-only update, and it
	// is retired when Reconcile removes or rebuilds the OLT.
	lcOnce sync.Once
	lc     *entryLifecycle
}

// entryLifecycle tracks the background workers bound to one OLT stack. The
// context is canceled when the stack is retired, and the SNMP pool is closed
// only after every worker has returned, so a long-running job such as the
// cache pre-warm never runs against a closed pool.
type entryLifecycle struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
	active  atomic.Int32
}

// lifecycle returns the entry's lifecycle, creating it on first use. Safe for
// concurrent callers.
func (e *OLTEntry) lifecycle() *entryLifecycle {
	e.lcOnce.Do(func() {
		if e.lc == nil {
			ctx, cancel := context.WithCancel(context.Background())
			e.lc = &entryLifecycle{ctx: ctx, cancel: cancel}
		}
	})
	return e.lc
}

// OLTRegistry is a thread-safe, dynamically-updatable registry of OLT runtime
// stacks. It replaces the static []oltRoute built at startup: new OLTs appear,
// changed OLTs reconnect, and removed OLTs clean up — all without a restart.
type OLTRegistry struct {
	mu         sync.RWMutex
	entries    map[string]*OLTEntry
	defaultOLT string
	cfg        *config.Config
	redisRepo  repository.OnuRedisRepositoryInterface
}

// NewOLTRegistry creates an empty registry. Call Reconcile to populate it with
// the initial OLT set, then StartPoller to keep it in sync with device-registry.
func NewOLTRegistry(cfg *config.Config, redisRepo repository.OnuRedisRepositoryInterface, defaultOLT string) *OLTRegistry {
	return &OLTRegistry{
		entries:    make(map[string]*OLTEntry),
		defaultOLT: defaultOLT,
		cfg:        cfg,
		redisRepo:  redisRepo,
	}
}

// Get returns the OLTEntry for the given OLT ID (read-locked). This is the hot
// path — every inbound request calls it via the resolveOLT middleware.
func (reg *OLTRegistry) Get(oltID string) (*OLTEntry, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	e, ok := reg.entries[oltID]
	return e, ok
}

// GetDefault returns the default OLT entry. Falls back to the first entry if
// the configured default is missing (e.g. it failed to init).
func (reg *OLTRegistry) GetDefault() (*OLTEntry, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	if e, ok := reg.entries[reg.defaultOLT]; ok {
		return e, true
	}
	// Fallback: return any entry (deterministic: sorted by ID).
	for _, e := range reg.entries {
		return e, true
	}
	return nil, false
}

// DefaultOLTID returns the configured default OLT identifier.
func (reg *OLTRegistry) DefaultOLTID() string {
	return reg.defaultOLT
}

// List returns the IDs of all registered OLTs (sorted, for deterministic output
// in health probes and logs).
func (reg *OLTRegistry) List() []string {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	ids := make([]string, 0, len(reg.entries))
	for id := range reg.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Len returns the number of registered OLTs.
func (reg *OLTRegistry) Len() int {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.entries)
}

// Reconcile compares the current registry against newOLTs and performs the
// minimum set of add/remove/update operations to converge. It is the core diff
// engine called both at startup (from the initial config) and on every poll tick.
//
// Change detection: an OLT is "updated" when its connection-relevant fields
// (host, port, community, walk, boards) change — requiring a full SNMP
// reconnection. Organization/user ID changes are applied in-place without
// reconnection.
func (reg *OLTRegistry) Reconcile(newOLTs []config.OLTRuntimeConfig) {
	// Build lookup of incoming OLTs.
	incoming := make(map[string]config.OLTRuntimeConfig, len(newOLTs))
	for _, o := range newOLTs {
		incoming[o.ID] = o
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()

	// Remove OLTs no longer in the incoming set.
	for id, entry := range reg.entries {
		if _, exists := incoming[id]; !exists {
			reg.removeOLTLocked(id, entry)
		}
	}

	// Add or update OLTs from the incoming set.
	for _, o := range newOLTs {
		existing, exists := reg.entries[o.ID]
		if !exists {
			// New OLT — create full stack.
			if err := reg.addOLTLocked(o); err != nil {
				logger.Error("olt_add_failed",
					zap.String("olt_id", o.ID),
					zap.String("host", o.Host),
					zap.Error(err))
			}
			continue
		}

		// Existing OLT — check if connection params changed.
		if oltChangeKey(existing.OLT) != oltChangeKey(o) {
			// Connection-relevant fields changed → full rebuild.
			logger.Info("olt_updated",
				zap.String("olt_id", o.ID),
				zap.String("old_host", existing.OLT.Host),
				zap.String("new_host", o.Host))
			reg.removeOLTLocked(o.ID, existing)
			if err := reg.addOLTLocked(o); err != nil {
				logger.Error("olt_update_failed",
					zap.String("olt_id", o.ID),
					zap.Error(err))
			}
			continue
		}

		// Connection params unchanged: apply metadata (user/org ID may have
		// changed in device-registry) without reconnecting. The entry is
		// replaced by an updated copy rather than mutated, because request
		// handlers read entries without holding reg.mu. The copy keeps the
		// same SNMP stack and lifecycle.
		reg.entries[o.ID] = &OLTEntry{
			OLT:       o,
			Repo:      existing.Repo,
			UC:        existing.UC,
			Handler:   existing.Handler,
			BoardPons: existing.BoardPons,
			UserID:    o.UserID,
			lc:        existing.lifecycle(),
		}
	}
}

// addOLTLocked creates the full per-OLT stack (SNMP conn → repo → usecase →
// handler) and stores it in the registry. Caller must hold reg.mu write lock.
func (reg *OLTRegistry) addOLTLocked(olt config.OLTRuntimeConfig) error {
	snmpConn, err := snmp.SetupSnmpConnectionWith(olt.Host, olt.Port, olt.Community)
	if err != nil {
		return fmt.Errorf("snmp connect %s:%d: %w", olt.Host, olt.Port, err)
	}

	snmpRepo := repository.NewPonRepositoryWithConcurrency(snmpConn, olt.MaxConcurrent, olt.UseWalk)

	cachePrefix := olt.ID
	if olt.ID == reg.defaultOLT {
		cachePrefix = "" // default OLT keeps unprefixed keys (back-compat)
	}
	uc := usecase.NewOnuUsecaseForOLT(snmpRepo, reg.redisRepo, reg.cfg.ForOLT(olt), cachePrefix)
	h := handler.NewOnuHandler(uc)

	reg.entries[olt.ID] = &OLTEntry{
		OLT:       olt,
		Repo:      snmpRepo,
		UC:        uc,
		Handler:   h,
		BoardPons: olt.BoardPons,
		UserID:    olt.UserID,
	}

	logger.Info("olt_added",
		zap.String("olt_id", olt.ID),
		zap.String("host", olt.Host),
		zap.Uint16("port", olt.Port),
		zap.Ints("boards", olt.Boards),
		zap.Bool("default", olt.ID == reg.defaultOLT))

	return nil
}

// removeOLTLocked deletes the entry from the registry and retires it: workers
// bound to the entry are canceled and its SNMP pool is closed once they have
// returned. Caller must hold reg.mu write lock.
func (reg *OLTRegistry) removeOLTLocked(id string, entry *OLTEntry) {
	retireEntry(entry)
	delete(reg.entries, id)

	logger.Info("olt_removed",
		zap.String("olt_id", id),
		zap.String("host", entry.OLT.Host))
}

// retireEntry cancels the entry's workers and closes its SNMP pool. When no
// worker is running the pool is closed immediately; otherwise it is closed in
// the background after the last worker returns, so an in-flight job never sees
// "SNMP pool closed". No new worker can be added once the entry has left the
// registry map, because GoForEach only starts workers for mapped entries under
// the read lock.
func retireEntry(entry *OLTEntry) {
	lc := entry.lifecycle()
	lc.cancel()
	if lc.active.Load() == 0 {
		entry.Repo.Close()
		return
	}
	go func() {
		lc.workers.Wait()
		entry.Repo.Close()
	}()
}

// GoForEach starts fn in its own goroutine for every registered OLT. Each run
// is bound to that entry's lifetime: the ctx passed to fn is canceled when
// parent is done or when Reconcile removes or rebuilds the entry, and the
// entry's SNMP pool is not closed until fn has returned. Use it for
// long-running per-OLT background work such as the cache pre-warm.
func (reg *OLTRegistry) GoForEach(parent context.Context, fn func(ctx context.Context, e *OLTEntry)) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, e := range reg.entries {
		lc := e.lifecycle()
		ctx, cancel := context.WithCancel(parent)
		stop := context.AfterFunc(lc.ctx, cancel)
		lc.active.Add(1)
		lc.workers.Add(1)
		go func(e *OLTEntry) {
			defer lc.workers.Done()
			defer lc.active.Add(-1)
			defer stop()
			defer cancel()
			fn(ctx, e)
		}(e)
	}
}

// StartPreWarm pre-warms the Redis cache of every registered OLT in the
// background. Each pre-warm stops early when ctx is done or when its OLT is
// removed or rebuilt by Reconcile, and never runs against a closed SNMP pool.
func (reg *OLTRegistry) StartPreWarm(ctx context.Context) {
	reg.GoForEach(ctx, func(ctx context.Context, e *OLTEntry) {
		e.UC.PreWarmCache(ctx)
	})
}

// StartPoller runs a background goroutine that fetches the OLT list from
// device-registry at the given interval and reconciles the registry. It blocks
// until ctx is canceled. Poll failures are logged but never wipe the registry
// — the current OLTs keep serving.
func (reg *OLTRegistry) StartPoller(ctx context.Context, registryURL, apiKey string, interval time.Duration) {
	if interval <= 0 {
		logger.Info("registry_poll_disabled")
		return
	}

	logger.Info("registry_poll_started",
		zap.String("url", registryURL),
		zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("registry_poll_stopped")
			return
		case <-ticker.C:
			olts, err := config.FetchRegistryOLTConfigs(registryURL, apiKey)
			if err != nil {
				logger.Warn("registry_poll_failed, keeping current OLTs",
					zap.Error(err))
				continue
			}
			if olts == nil {
				logger.Warn("registry_poll_empty, keeping current OLTs")
				continue
			}
			reg.Reconcile(olts)
		}
	}
}

// Close tears down all OLT stacks. Each stack is retired like a removal:
// background workers are canceled and the SNMP pool is closed immediately when
// none is running, otherwise as soon as the last one returns, so shutdown is
// never delayed by an in-flight SNMP walk. Safe to call multiple times
// (subsequent calls are no-ops on an empty map).
func (reg *OLTRegistry) Close() {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	for id, entry := range reg.entries {
		retireEntry(entry)
		logger.Info("olt_closed", zap.String("olt_id", id))
		delete(reg.entries, id)
	}
}

// HealthCheck pings every registered OLT's SNMP connection. Returns the first
// error encountered (with the OLT ID in the message) or nil if all are healthy.
func (reg *OLTRegistry) HealthCheck(ctx context.Context) error {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for id, entry := range reg.entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := entry.Repo.Ping(); err != nil {
			return fmt.Errorf("olt %s: %w", id, err)
		}
	}
	return nil
}

// oltChangeKey builds a string that uniquely identifies the connection-relevant
// configuration of an OLT. When this key changes between the current and
// incoming config, the SNMP connection must be rebuilt.
func oltChangeKey(o config.OLTRuntimeConfig) string {
	// Sort board specs for deterministic comparison.
	boards := make([]string, 0, len(o.BoardPons))
	for slot, pons := range o.BoardPons {
		boards = append(boards, fmt.Sprintf("%d:%d", slot, pons))
	}
	sort.Strings(boards)

	return fmt.Sprintf("%s|%d|%s|%t|%s",
		o.Host, o.Port, o.Community, o.UseWalk, strings.Join(boards, ","))
}
