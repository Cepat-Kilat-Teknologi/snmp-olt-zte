package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func legacyOK() OLTRuntimeConfig {
	boards, bp := parseBoardSpecs("1,2", MaxPonID)
	m, _ := InitializeBoardPonMapFromSpecs(bp)
	return OLTRuntimeConfig{
		ID: "default", Host: "10.0.0.1", Port: 161, Community: "public",
		MaxConcurrent: 5, Boards: boards, BoardPons: bp, PonsPerBoard: 16, BoardPonMap: m,
	}
}

func TestBuildOLTRegistry_Legacy(t *testing.T) {
	olts, def, err := buildOLTRegistry("", "", legacyOK())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(olts) != 1 || olts[0].ID != "default" || def != "default" {
		t.Fatalf("got %d olts, default=%q", len(olts), def)
	}
}

func TestBuildOLTRegistry_LegacyMissingHost(t *testing.T) {
	l := legacyOK()
	l.Host = ""
	if _, _, err := buildOLTRegistry("", "", l); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestBuildOLTRegistry_MultiMixedCards(t *testing.T) {
	js := `[
		{"id":"c320","host":"10.0.0.1","community":"public","boards":"1,2"},
		{"id":"c300a","host":"192.0.2.20","port":1161,"community":"public","boards":"3:16,5:8"}
	]`
	olts, def, err := buildOLTRegistry(js, "", legacyOK())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(olts) != 2 {
		t.Fatalf("got %d olts, want 2", len(olts))
	}
	if def != "c320" {
		t.Errorf("default=%q, want c320 (first)", def)
	}

	c300 := olts[1]
	if c300.ID != "c300a" || c300.Port != 1161 || c300.Community != "public" {
		t.Errorf("c300a parsed wrong: %+v", c300)
	}
	// Per-slot PON counts: slot 3 GTGH (16), slot 5 GTGO (8).
	if c300.BoardPons[3] != 16 || c300.BoardPons[5] != 8 {
		t.Errorf("c300a BoardPons = %v, want {3:16, 5:8}", c300.BoardPons)
	}
	// Map must contain slot5/pon8 but NOT slot5/pon9 (GTGO has only 8 PONs).
	if _, ok := c300.BoardPonMap[BoardPonKey{BoardID: 5, PonID: 8}]; !ok {
		t.Error("missing slot 5 pon 8")
	}
	if _, ok := c300.BoardPonMap[BoardPonKey{BoardID: 5, PonID: 9}]; ok {
		t.Error("slot 5 pon 9 must not exist on an 8-port GTGO")
	}
	if _, ok := c300.BoardPonMap[BoardPonKey{BoardID: 3, PonID: 16}]; !ok {
		t.Error("missing slot 3 pon 16 (GTGH)")
	}
}

func TestBuildOLTRegistry_Errors(t *testing.T) {
	cases := map[string]string{
		"invalid json":      `{nope`,
		"empty array":       `[]`,
		"missing host":      `[{"id":"x","community":"c"}]`,
		"missing community": `[{"id":"x","host":"h"}]`,
		"missing id":        `[{"host":"h","community":"c"}]`,
		"duplicate id":      `[{"id":"a","host":"h","community":"c"},{"id":"a","host":"h2","community":"c"}]`,
		"bad id":            `[{"id":"a/b","host":"h","community":"c"}]`,
	}
	for name, js := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := buildOLTRegistry(js, "", legacyOK()); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}

func TestBuildOLTRegistry_DefaultOLT(t *testing.T) {
	js := `[{"id":"a","host":"h","community":"c"},{"id":"b","host":"h2","community":"c"}]`
	_, def, err := buildOLTRegistry(js, "b", legacyOK())
	if err != nil || def != "b" {
		t.Fatalf("default override failed: def=%q err=%v", def, err)
	}
	if _, _, err := buildOLTRegistry(js, "nope", legacyOK()); err == nil {
		t.Error("expected error for unknown DEFAULT_OLT")
	}
}

func TestBuildOLTRegistry_LegacyDefaultsID(t *testing.T) {
	l := legacyOK()
	l.ID = "" // an unnamed legacy OLT must be assigned the id "default"
	olts, def, err := buildOLTRegistry("", "", l)
	if err != nil {
		t.Fatal(err)
	}
	if olts[0].ID != "default" || def != "default" {
		t.Errorf("empty legacy id should default to 'default'; got id=%q def=%q", olts[0].ID, def)
	}
}

func TestBuildOLTRegistry_MaxConcurrentFallback(t *testing.T) {
	l := legacyOK()
	l.MaxConcurrent = 0 // no usable default → oltFromJSON must fall back to 5
	js := `[{"id":"x","host":"h","community":"c"}]`
	olts, _, err := buildOLTRegistry(js, "", l)
	if err != nil {
		t.Fatal(err)
	}
	if olts[0].MaxConcurrent != 5 {
		t.Errorf("maxConcurrent=%d, want fallback 5", olts[0].MaxConcurrent)
	}
}

func TestBuildOLTRegistry_Defaults(t *testing.T) {
	js := `[{"id":"x","host":"h","community":"c"}]` // no port/boards/pons/maxConcurrent
	olts, _, err := buildOLTRegistry(js, "", legacyOK())
	if err != nil {
		t.Fatal(err)
	}
	o := olts[0]
	if o.Port != 161 {
		t.Errorf("port=%d, want default 161", o.Port)
	}
	if o.MaxConcurrent != 5 {
		t.Errorf("maxConcurrent=%d, want 5 (legacy default)", o.MaxConcurrent)
	}
	if !reflect.DeepEqual(o.Boards, []int{1, 2}) {
		t.Errorf("boards=%v, want default [1 2]", o.Boards)
	}
	if o.BoardPons[1] != 16 {
		t.Errorf("pons[1]=%d, want default 16", o.BoardPons[1])
	}
}

func TestInitializeBoardPonMapFromSpecs_EmptyFallback(t *testing.T) {
	m, err := InitializeBoardPonMapFromSpecs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 32 { // default {1,2} x 16
		t.Errorf("empty specs -> %d entries, want 32 (default {1,2}x16)", len(m))
	}
}

func TestInitializeBoardPonMapFromSpecs_InvalidSlotErrors(t *testing.T) {
	// A slot beyond MaxBoardID makes GenerateBoardPonOID fail; the error must
	// propagate (defensive path; parseBoardSpecs normally sanitizes such input).
	if _, err := InitializeBoardPonMapFromSpecs(map[int]int{9999: 1}); err == nil {
		t.Fatal("expected error for out-of-range slot")
	}
}

func TestLoadConfig_MultiOLT(t *testing.T) {
	t.Setenv("SNMP_HOST", "") // legacy SNMP_* not required when OLTS is set
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("OLTS", `[
		{"id":"c320","host":"10.0.0.1","community":"public","boards":"1,2"},
		{"id":"c300a","host":"1.2.3.4","port":1161,"community":"public","boards":"3:16,5:8"}
	]`)
	t.Setenv("DEFAULT_OLT", "c300a")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.OLTs) != 2 {
		t.Fatalf("got %d OLTs, want 2", len(cfg.OLTs))
	}
	if cfg.DefaultOLT != "c300a" {
		t.Errorf("DefaultOLT=%q, want c300a", cfg.DefaultOLT)
	}
	// Legacy mirror fields reflect the default OLT (c300a).
	if cfg.SnmpCfg.IP != "1.2.3.4" || cfg.SnmpCfg.Port != 1161 {
		t.Errorf("mirror SnmpCfg=%+v, want host 1.2.3.4:1161", cfg.SnmpCfg)
	}
	if cfg.BoardPons[5] != 8 {
		t.Errorf("mirror BoardPons[5]=%d, want 8 (GTGO)", cfg.BoardPons[5])
	}
}

func TestParseBoardSpecs(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		defaultPons int
		wantBoards  []int
		wantPons    map[int]int
	}{
		{"uniform default", "1,2", 16, []int{1, 2}, map[int]int{1: 16, 2: 16}},
		{"mixed cards", "3:16,5:8", 16, []int{3, 5}, map[int]int{3: 16, 5: 8}},
		{"bare uses default", "3:16,5", 16, []int{3, 5}, map[int]int{3: 16, 5: 16}},
		{"empty falls back", "", 16, []int{1, 2}, map[int]int{1: 16, 2: 16}},
		{"bad pons -> default", "3:99", 8, []int{3}, map[int]int{3: 8}},
		{"skip bad slot", "0,5:8", 16, []int{5}, map[int]int{5: 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boards, pons := parseBoardSpecs(tt.in, tt.defaultPons)
			if !reflect.DeepEqual(boards, tt.wantBoards) {
				t.Errorf("boards = %v, want %v", boards, tt.wantBoards)
			}
			if !reflect.DeepEqual(pons, tt.wantPons) {
				t.Errorf("pons = %v, want %v", pons, tt.wantPons)
			}
		})
	}
}

func TestForOLT(t *testing.T) {
	base := &Config{
		OltCfg:   OltConfig{BaseOID1: ".1.3.6", BaseOID2: ".1.3.7"},
		CacheCfg: CacheConfig{ONUInfoTTL: 1800},
	}
	boards, bp := parseBoardSpecs("3:16,5:8", 16)
	m, _ := InitializeBoardPonMapFromSpecs(bp)
	o := OLTRuntimeConfig{ID: "c300a", Host: "1.2.3.4", Port: 1161, Community: "ro", Boards: boards, BoardPons: bp, BoardPonMap: m}

	got := base.ForOLT(o)
	if got.SnmpCfg.IP != "1.2.3.4" || got.SnmpCfg.Port != 1161 || got.SnmpCfg.Community != "ro" {
		t.Errorf("SnmpCfg not scoped to OLT: %+v", got.SnmpCfg)
	}
	if got.OltCfg.BaseOID1 != ".1.3.6" || got.CacheCfg.ONUInfoTTL != 1800 {
		t.Error("shared sub-configs should be preserved")
	}
	if !reflect.DeepEqual(got.BoardPons, bp) {
		t.Errorf("BoardPons = %v, want %v", got.BoardPons, bp)
	}
}

func TestResolveOLTSJSON_InlineWins(t *testing.T) {
	got, err := resolveOLTSJSON(`[{"id":"x"}]`, "/nonexistent/should-be-ignored.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `[{"id":"x"}]` {
		t.Errorf("inline OLTS must win over file; got %q", got)
	}
}

func TestResolveOLTSJSON_Neither(t *testing.T) {
	got, err := resolveOLTSJSON("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("no source set must return empty (legacy mode); got %q", got)
	}
}

func TestResolveOLTSJSON_FromFile(t *testing.T) {
	js := `[{"id":"c320","host":"10.0.0.1","community":"public","boards":"1,2"}]`
	f := filepath.Join(t.TempDir(), "olts.json")
	if err := os.WriteFile(f, []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveOLTSJSON("", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != js {
		t.Errorf("file contents mismatch; got %q", got)
	}
}

func TestResolveOLTSJSON_MissingFileFailsFast(t *testing.T) {
	if _, err := resolveOLTSJSON("", "/nonexistent/olts.json"); err == nil {
		t.Fatal("expected fail-fast error for missing OLTS_FILE")
	}
}

func TestResolveOLTSJSON_EmptyFileFailsFast(t *testing.T) {
	f := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(f, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveOLTSJSON("", f); err == nil {
		t.Fatal("expected fail-fast error for empty OLTS_FILE")
	}
}

// TestLoadConfig_OLTSFile exercises the full env→file→registry path that
// `OLTS_FILE` enables (the form used when the registry is mounted as a
// Kubernetes Secret). It mirrors TestLoadConfig_MultiOLT but sources the
// JSON from a file instead of the inline OLTS var.
func TestLoadConfig_OLTSFile(t *testing.T) {
	js := `[
		{"id":"c320","host":"10.0.0.1","community":"public","boards":"1,2"},
		{"id":"c300a","host":"1.2.3.4","port":1161,"community":"public","boards":"3:16,5:8"}
	]`
	f := filepath.Join(t.TempDir(), "olts.json")
	if err := os.WriteFile(f, []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNMP_HOST", "") // legacy SNMP_* not required when OLTS_FILE is set
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("OLTS", "") // no inline value — must fall through to the file
	t.Setenv("OLTS_FILE", f)
	t.Setenv("DEFAULT_OLT", "c300a")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.OLTs) != 2 {
		t.Fatalf("got %d OLTs, want 2", len(cfg.OLTs))
	}
	if cfg.DefaultOLT != "c300a" {
		t.Errorf("DefaultOLT=%q, want c300a", cfg.DefaultOLT)
	}
	// Mirror fields must reflect the default OLT loaded from the file.
	if cfg.SnmpCfg.IP != "1.2.3.4" || cfg.SnmpCfg.Port != 1161 {
		t.Errorf("mirror SnmpCfg=%+v, want host 1.2.3.4:1161", cfg.SnmpCfg)
	}
	if cfg.BoardPons[5] != 8 {
		t.Errorf("mirror BoardPons[5]=%d, want 8 (GTGO)", cfg.BoardPons[5])
	}
}

// TestLoadConfig_OLTSInlineWinsOverFile verifies the documented precedence:
// an inline OLTS value is used even when OLTS_FILE also points at a (different)
// valid file. The file here would yield a single OLT "fromfile"; the inline
// value yields "inline" — we assert the inline one wins.
func TestLoadConfig_OLTSInlineWinsOverFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "olts.json")
	if err := os.WriteFile(f, []byte(`[{"id":"fromfile","host":"9.9.9.9","community":"c"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("OLTS", `[{"id":"inline","host":"10.0.0.1","community":"public"}]`)
	t.Setenv("OLTS_FILE", f)
	t.Setenv("DEFAULT_OLT", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.OLTs) != 1 || cfg.OLTs[0].ID != "inline" {
		t.Fatalf("inline OLTS must win over OLTS_FILE; got %+v", cfg.OLTs)
	}
}

// TestLoadConfig_OLTSFileMissingFailsFast asserts the fail-fast contract at the
// LoadConfig boundary: a configured-but-unreadable OLTS_FILE aborts startup
// rather than silently falling back to legacy SNMP_*.
func TestLoadConfig_OLTSFileMissingFailsFast(t *testing.T) {
	t.Setenv("SNMP_HOST", "10.0.0.1") // legacy vars present, but must NOT be used
	t.Setenv("SNMP_COMMUNITY", "public")
	t.Setenv("OLTS", "")
	t.Setenv("OLTS_FILE", filepath.Join(t.TempDir(), "does-not-exist.json"))
	t.Setenv("DEFAULT_OLT", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected LoadConfig to fail fast on a missing OLTS_FILE")
	}
}
