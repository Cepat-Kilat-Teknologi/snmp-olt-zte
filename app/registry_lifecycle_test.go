package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/usecase"
	"github.com/alicebob/miniredis/v2"
	"github.com/gosnmp/gosnmp"
	rds "github.com/redis/go-redis/v9"
)

// poolSnmpRepo emulates the closed-pool semantics of the real snmpRepository:
// once Close has been called every operation fails with "SNMP pool closed".
// The first BulkWalk blocks until release is closed, standing in for a slow
// walk that is still in flight when the device-registry poller rebuilds the OLT.
type poolSnmpRepo struct {
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	closeOnce   sync.Once
	closedCh    chan struct{}
	closed      atomic.Bool
	callsClosed atomic.Int32 // SNMP calls made after Close (each one logs "SNMP pool closed" in production)
}

func newPoolSnmpRepo() *poolSnmpRepo {
	return &poolSnmpRepo{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		closedCh: make(chan struct{}),
	}
}

var errPoolClosed = errors.New("SNMP pool closed")

func (r *poolSnmpRepo) op() error {
	if r.closed.Load() {
		r.callsClosed.Add(1)
		return errPoolClosed
	}
	return nil
}

func (r *poolSnmpRepo) Get([]string) (*gosnmp.SnmpPacket, error) {
	if err := r.op(); err != nil {
		return nil, err
	}
	return &gosnmp.SnmpPacket{}, nil
}

func (r *poolSnmpRepo) Walk(string, func(gosnmp.SnmpPDU) error) error { return r.op() }

func (r *poolSnmpRepo) BulkWalk(string, func(gosnmp.SnmpPDU) error) error {
	if err := r.op(); err != nil {
		return err
	}
	first := false
	r.startOnce.Do(func() { first = true; close(r.started) })
	if first {
		<-r.release
	}
	return nil
}

func (r *poolSnmpRepo) Ping() error { return r.op() }

func (r *poolSnmpRepo) Close() {
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		close(r.closedCh)
	})
}

// prewarmEntry builds an OLT entry backed by the real ONU usecase (so the
// pre-warm runs its production loop) on top of repo and a miniredis cache.
func prewarmEntry(t *testing.T, repo repository.SnmpRepositoryInterface) *OLTEntry {
	t.Helper()
	mr := miniredis.RunT(t)
	client := rds.NewClient(&rds.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	boardPons := map[int]int{1: 8}
	bpMap := make(map[config.BoardPonKey]*config.BoardPonConfig)
	for p := 1; p <= 8; p++ {
		bpMap[config.BoardPonKey{BoardID: 1, PonID: p}] = &config.BoardPonConfig{OnuIDNameOID: ".1.1.1"}
	}
	cfg := &config.Config{
		OltCfg:      config.OltConfig{BaseOID1: "1.3.6.1.4.1", BaseOID2: "1.3.6.1.4.2"},
		CacheCfg:    config.CacheConfig{ONUInfoTTL: 600, ONUDetailTTL: 300, EmptyOnuIDTTL: 300},
		BoardPons:   boardPons,
		BoardPonMap: bpMap,
	}

	e := testOLTEntry("c320", "10.0.0.1", 1, boardPons)
	e.Repo = repo
	e.UC = usecase.NewOnuUsecaseForOLT(repo, repository.NewOnuRedisRepo(client), cfg, "")
	return e
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// Regression: at startup the cache pre-warm ran against the OLT entry built
// from the initial config. When the first device-registry poll rebuilt that
// OLT, Reconcile closed the old SNMP pool while the pre-warm was still
// iterating, so every remaining board/PON failed with "SNMP pool closed"
// (get_onu_information_failed bursts in the first minute after pod start).
func TestOLTRegistry_PreWarmSurvivesRebuild_NoClosedPoolCalls(t *testing.T) {
	repo := newPoolSnmpRepo()
	reg := testRegistry("c320", prewarmEntry(t, repo))
	defer reg.Close()

	reg.StartPreWarm(context.Background())
	waitClosed(t, repo.started, "pre-warm to start its first SNMP walk")

	// First registry poll: connection-relevant fields changed, so the OLT is
	// rebuilt while the pre-warm walk is still in flight.
	reg.Reconcile([]config.OLTRuntimeConfig{
		{ID: "c320", Host: "10.0.0.99", Port: 161, Community: "public", BoardPons: map[int]int{1: 8}},
	})
	if got, ok := reg.Get("c320"); !ok || got.Repo == repository.SnmpRepositoryInterface(repo) {
		t.Fatal("c320 should be rebuilt with a new SNMP repo")
	}

	close(repo.release)
	// The old pool must still be closed (no leak) once the pre-warm stops.
	waitClosed(t, repo.closedCh, "old SNMP pool to be closed")
	// Give a pre-warm that kept running against the closed pool time to show up.
	time.Sleep(200 * time.Millisecond)

	if n := repo.callsClosed.Load(); n != 0 {
		t.Fatalf("pre-warm made %d SNMP calls on a closed pool, want 0", n)
	}
}

func TestOLTRegistry_RemoveWithoutWorkersClosesImmediately(t *testing.T) {
	repo := newPoolSnmpRepo()
	reg := testRegistry("c320", prewarmEntry(t, repo))

	reg.Reconcile(nil)

	if !repo.closed.Load() {
		t.Fatal("a removed OLT with no running worker should be closed synchronously")
	}
}

func TestOLTRegistry_GoForEach_ParentCancelStopsWorker(t *testing.T) {
	e := testOLTEntry("c320", "10.0.0.1", 1, nil)
	reg := testRegistry("c320", e)
	defer reg.Close()

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	reg.GoForEach(ctx, func(ctx context.Context, _ *OLTEntry) {
		<-ctx.Done()
		close(stopped)
	})
	cancel()
	waitClosed(t, stopped, "worker to observe parent cancellation")
}

func TestOLTRegistry_Close_DefersPoolCloseUntilWorkerReturns(t *testing.T) {
	repo := newPoolSnmpRepo()
	e := testOLTEntry("c320", "10.0.0.1", 1, nil)
	e.Repo = repo
	reg := testRegistry("c320", e)

	canceled := make(chan struct{})
	finish := make(chan struct{})
	reg.GoForEach(context.Background(), func(ctx context.Context, _ *OLTEntry) {
		<-ctx.Done() // Close cancels the worker
		close(canceled)
		<-finish
	})

	reg.Close() // must not block on the running worker
	waitClosed(t, canceled, "worker to be canceled by Close")
	if repo.closed.Load() {
		t.Fatal("pool was closed while the worker was still running")
	}
	if reg.Len() != 0 {
		t.Fatal("Close should empty the registry immediately")
	}

	close(finish)
	waitClosed(t, repo.closedCh, "pool to be closed after the worker returns")
}

func TestOLTRegistry_MetadataUpdateKeepsWorkerLifecycle(t *testing.T) {
	repo := newPoolSnmpRepo()
	e := testOLTEntry("c320", "10.0.0.1", 1, map[int]int{1: 16})
	e.Repo = repo
	reg := testRegistry("c320", e)

	finish := make(chan struct{})
	reg.GoForEach(context.Background(), func(ctx context.Context, _ *OLTEntry) {
		<-ctx.Done()
		<-finish
	})

	// Metadata-only change: the entry is replaced by a copy that shares the
	// SNMP stack and the lifecycle, so the worker keeps running.
	reg.Reconcile([]config.OLTRuntimeConfig{
		{ID: "c320", Host: "10.0.0.1", Port: 161, Community: "public", UserID: 42, BoardPons: map[int]int{1: 16}},
	})
	got, _ := reg.Get("c320")
	if got == e || got.UserID != 42 || got.Repo != repository.SnmpRepositoryInterface(repo) {
		t.Fatalf("expected an updated copy sharing the repo, got %+v", got)
	}

	// Removing the updated copy must still cancel the original worker and
	// close the pool only after it returns.
	reg.Reconcile(nil)
	if repo.closed.Load() {
		t.Fatal("pool closed while the worker was still running")
	}
	close(finish)
	waitClosed(t, repo.closedCh, "pool to be closed after the worker returns")
}

// ---------------------------------------------------------------------------
// defaultOLTFetcher
// ---------------------------------------------------------------------------

type recordingUsecase struct {
	mockOnuUsecase
	calls atomic.Int32
}

func (u *recordingUsecase) GetByBoardIDAndPonID(context.Context, int, int) ([]model.ONUInfoPerBoard, error) {
	u.calls.Add(1)
	return []model.ONUInfoPerBoard{{ID: 1}}, nil
}

func (u *recordingUsecase) GetByBoardIDPonIDAndOnuID(context.Context, int, int, int) (model.ONUCustomerInfo, error) {
	u.calls.Add(1)
	return model.ONUCustomerInfo{ID: 7}, nil
}

func (u *recordingUsecase) InvalidateONUCache(context.Context, int, int, int) error {
	u.calls.Add(1)
	return nil
}

func TestDefaultOLTFetcher_FollowsRebuiltDefault(t *testing.T) {
	first := &recordingUsecase{}
	e := testOLTEntry("c320", "10.0.0.1", 1, nil)
	e.UC = first
	reg := testRegistry("c320", e)
	f := defaultOLTFetcher{reg: reg}

	if _, err := f.GetByBoardIDAndPonID(context.Background(), 1, 1); err != nil {
		t.Fatal(err)
	}

	// Swap the default entry, as Reconcile does on a rebuild.
	second := &recordingUsecase{}
	e2 := testOLTEntry("c320", "10.0.0.99", 1, nil)
	e2.UC = second
	reg.mu.Lock()
	reg.entries["c320"] = e2
	reg.mu.Unlock()

	ctx := context.Background()
	if _, err := f.GetByBoardIDAndPonID(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	if info, err := f.GetByBoardIDPonIDAndOnuID(ctx, 1, 1, 7); err != nil || info.ID != 7 {
		t.Fatalf("GetByBoardIDPonIDAndOnuID = %+v, %v", info, err)
	}
	if err := f.InvalidateONUCache(ctx, 1, 1, 7); err != nil {
		t.Fatal(err)
	}
	if first.calls.Load() != 1 || second.calls.Load() != 3 {
		t.Fatalf("calls: first=%d second=%d, want 1 and 3", first.calls.Load(), second.calls.Load())
	}
}

func TestDefaultOLTFetcher_NoDefault(t *testing.T) {
	f := defaultOLTFetcher{reg: testRegistry("c320")}
	ctx := context.Background()
	if _, err := f.GetByBoardIDAndPonID(ctx, 1, 1); !errors.Is(err, errNoDefaultOLT) {
		t.Fatalf("GetByBoardIDAndPonID err = %v", err)
	}
	if _, err := f.GetByBoardIDPonIDAndOnuID(ctx, 1, 1, 1); !errors.Is(err, errNoDefaultOLT) {
		t.Fatalf("GetByBoardIDPonIDAndOnuID err = %v", err)
	}
	if err := f.InvalidateONUCache(ctx, 1, 1, 1); !errors.Is(err, errNoDefaultOLT) {
		t.Fatalf("InvalidateONUCache err = %v", err)
	}
}
