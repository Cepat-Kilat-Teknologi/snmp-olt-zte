package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRegisterOptional_NonGating verifies that a failing optional probe is
// reported as down but does NOT flip the overall readiness flag (used for
// per-OLT SNMP probes in multi-OLT mode).
func TestRegisterOptional_NonGating(t *testing.T) {
	c := NewChecker(time.Second)
	c.Register("redis", time.Minute, func(context.Context) error { return nil })
	c.RegisterOptional("snmp_c300a", time.Minute, func(context.Context) error { return errors.New("unreachable") })

	statuses, healthy := c.Check(context.Background())
	if !healthy {
		t.Error("optional probe failure must not flip overall healthy")
	}

	var found bool
	for _, s := range statuses {
		if s.Name == "snmp_c300a" {
			found = true
			if s.State != "down" {
				t.Errorf("optional probe state = %q, want down", s.State)
			}
		}
	}
	if !found {
		t.Error("optional probe missing from statuses")
	}
}

// TestRegister_CriticalGating verifies a failing critical probe flips readiness.
func TestRegister_CriticalGating(t *testing.T) {
	c := NewChecker(time.Second)
	c.Register("snmp", time.Minute, func(context.Context) error { return errors.New("down") })
	if _, healthy := c.Check(context.Background()); healthy {
		t.Error("critical probe failure must flip healthy=false")
	}
}
