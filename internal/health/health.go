// Package health provides readiness probes with cached dependency checks.
//
// Kubernetes / Docker / load balancers call /readyz on every pod frequently
// (often once per second). Hitting Redis or running an SNMP Get on every
// probe would be wasteful and slow down the instance. This package therefore
// memoises the result of each probe for a configurable TTL and only re-runs
// the probe when the cache expires.
package health

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Probe is a dependency check function. It should return nil if the
// dependency is reachable, or an error describing the failure. Probes run
// with a bounded context so they cannot hang the readyz endpoint.
type Probe func(ctx context.Context) error

// failureTTL caps how long a FAILED probe result is served from cache. Without
// it every readyz call re-probes a down dependency — each caller paying the
// probe timeout while holding the dependency mutex, so concurrent readyz calls
// pile up behind one down OLT. 5s still detects recovery within a typical
// k8s probe interval while preventing that probe storm.
const failureTTL = 5 * time.Second

// dependency is a registered probe with its own TTL and cached state.
type dependency struct {
	name     string
	ttl      time.Duration
	probe    Probe
	critical bool // when false, a failure is reported but does not flip overall healthy

	mu      sync.Mutex
	lastAt  time.Time
	lastErr error
}

// Status is the outcome of a probe lookup (used when rendering readyz JSON).
type Status struct {
	Name  string `json:"-"`
	State string `json:"state"`           // "up" | "down"
	Err   string `json:"error,omitempty"` // populated when state=="down"
}

// Checker coordinates one or more dependency probes.
type Checker struct {
	timeout time.Duration
	deps    []*dependency
}

// NewChecker constructs a Checker. `timeout` bounds each individual probe
// call — use something small (e.g. 2s) so a slow dependency doesn't pin the
// readyz response.
func NewChecker(timeout time.Duration) *Checker {
	return &Checker{timeout: timeout}
}

// Register adds a critical probe. ttl controls how long a successful result is
// cached; a failure is cached for at most failureTTL so recovery is detected
// within a few seconds without re-probing a down dependency on every call.
// A failing critical probe flips the overall readiness flag to not-ready.
func (c *Checker) Register(name string, ttl time.Duration, probe Probe) {
	c.deps = append(c.deps, &dependency{
		name:     name,
		ttl:      ttl,
		probe:    probe,
		critical: true,
	})
}

// RegisterOptional adds a non-critical probe. Its status is reported in the
// readyz dependency map, but a failure does NOT flip overall readiness. Used
// for per-OLT SNMP reachability in multi-OLT mode: one unreachable OLT should
// surface as degraded, not take the whole instance out of rotation.
func (c *Checker) RegisterOptional(name string, ttl time.Duration, probe Probe) {
	c.deps = append(c.deps, &dependency{
		name:     name,
		ttl:      ttl,
		probe:    probe,
		critical: false,
	})
}

// Check runs all registered probes and returns per-dependency status plus
// an overall healthy flag. A successful result may be served from cache.
func (c *Checker) Check(ctx context.Context) (statuses []Status, healthy bool) {
	healthy = true
	now := time.Now()

	for _, d := range c.deps {
		d.mu.Lock()
		// Successes cache for the dependency's ttl; failures for at most
		// failureTTL (recover fast, but never re-probe a down dep per call).
		ttl := d.ttl
		if d.lastErr != nil && failureTTL < ttl {
			ttl = failureTTL
		}
		cached := !d.lastAt.IsZero() && now.Sub(d.lastAt) < ttl
		if !cached {
			probeCtx, cancel := context.WithTimeout(ctx, c.timeout)
			d.lastErr = d.probe(probeCtx)
			d.lastAt = time.Now()
			cancel()
		}
		err := d.lastErr
		d.mu.Unlock()

		s := Status{Name: d.name, State: "up"}
		if err != nil {
			s.State = "down"
			s.Err = err.Error()
			if d.critical {
				healthy = false
			}
		}
		statuses = append(statuses, s)
	}
	return statuses, healthy
}

// ErrNoProbes is returned by Check when the checker has no registered probes.
// This is treated as "unknown"; callers should usually register at least one
// probe so the readyz endpoint is meaningful.
var ErrNoProbes = errors.New("no dependency probes registered")
