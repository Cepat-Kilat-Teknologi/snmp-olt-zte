package buildinfo

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	// Defaults are safe when ldflags are not passed.
	if Version == "" || Commit == "" || BuildTime == "" || APIVersion == "" {
		t.Error("expected non-empty defaults for Version, Commit, BuildTime, APIVersion")
	}
}

func TestUptime(t *testing.T) {
	first := Uptime()
	time.Sleep(2 * time.Millisecond)
	second := Uptime()
	if second <= first {
		t.Errorf("expected Uptime to grow over time, got %v then %v", first, second)
	}
}

func TestInfo(t *testing.T) {
	// Override values to verify Info returns the current state rather than
	// captured initial values.
	prev := struct {
		v, c, bt, av string
	}{Version, Commit, BuildTime, APIVersion}
	defer func() {
		Version, Commit, BuildTime, APIVersion = prev.v, prev.c, prev.bt, prev.av
	}()

	Version = "9.9.9"
	Commit = "cafebabe"
	BuildTime = "2026-01-01T00:00:00Z"
	APIVersion = "v2"

	info := Info()
	if info["version"] != "9.9.9" {
		t.Errorf("version: got %v want 9.9.9", info["version"])
	}
	if info["commit"] != "cafebabe" {
		t.Errorf("commit: got %v want cafebabe", info["commit"])
	}
	if info["build_time"] != "2026-01-01T00:00:00Z" {
		t.Errorf("build_time: got %v want 2026-01-01T00:00:00Z", info["build_time"])
	}
	if info["api_version"] != "v2" {
		t.Errorf("api_version: got %v want v2", info["api_version"])
	}
	if _, ok := info["uptime"].(string); !ok {
		t.Errorf("uptime: expected string, got %T", info["uptime"])
	}
}
