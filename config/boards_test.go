package config

import (
	"reflect"
	"testing"
)

// TestParseBoards covers OLT_BOARDS parsing: valid lists, defaulting, trimming,
// de-duplication, sorting, and rejection of invalid / out-of-range entries.
func TestParseBoards(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []int
	}{
		{"c320 default pair", "1,2", []int{1, 2}},
		{"c300 slots", "3,5", []int{3, 5}},
		{"empty falls back to default", "", []int{1, 2}},
		{"whitespace trimmed", " 3 , 5 ", []int{3, 5}},
		{"dedup and sort", "5,3,3,5", []int{3, 5}},
		{"skip invalid and out-of-range, keep valid", "0,31,abc,2", []int{2}},
		{"all invalid falls back to default", "0,99,foo", []int{1, 2}},
		{"single slot", "3", []int{3}},
		{"max boundary kept", "30", []int{30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBoards(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBoards(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBoardSet verifies the lookup-set helper, including the DefaultBoards fallback.
func TestBoardSet(t *testing.T) {
	c := &Config{Boards: []int{3, 5}}
	set := c.BoardSet()
	if len(set) != 2 || !set[3] || !set[5] {
		t.Errorf("BoardSet() = %v, want {3,5}", set)
	}
	if set[1] {
		t.Error("BoardSet() must not contain slot 1 for C300 boards {3,5}")
	}

	// Empty Boards falls back to DefaultBoards {1,2}.
	empty := (&Config{}).BoardSet()
	if len(empty) != 2 || !empty[1] || !empty[2] {
		t.Errorf("BoardSet() fallback = %v, want {1,2}", empty)
	}
}

// TestLoadConfig_OLTBoards verifies that OLT_BOARDS / OLT_PONS_PER_BOARD flow
// into the generated BoardPonMap, and that an out-of-range PON count is clamped.
func TestLoadConfig_OLTBoards(t *testing.T) {
	t.Setenv("SNMP_HOST", "10.0.0.1")
	t.Setenv("SNMP_COMMUNITY", "public")
	t.Setenv("OLT_BOARDS", "3,5")
	t.Setenv("OLT_PONS_PER_BOARD", "99") // out of range -> clamped to MaxPonID (16)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if !reflect.DeepEqual(cfg.Boards, []int{3, 5}) {
		t.Errorf("Boards = %v, want [3 5]", cfg.Boards)
	}
	if cfg.PonsPerBoard != MaxPonID {
		t.Errorf("PonsPerBoard = %d, want %d (clamped)", cfg.PonsPerBoard, MaxPonID)
	}
	if _, ok := cfg.BoardPonMap[BoardPonKey{BoardID: 3, PonID: 1}]; !ok {
		t.Error("expected BoardPonMap to contain slot 3 pon 1")
	}
	if _, ok := cfg.BoardPonMap[BoardPonKey{BoardID: 1, PonID: 1}]; ok {
		t.Error("BoardPonMap must not contain slot 1 for OLT_BOARDS=3,5")
	}
	if len(cfg.BoardPonMap) != 2*MaxPonID {
		t.Errorf("BoardPonMap size = %d, want %d", len(cfg.BoardPonMap), 2*MaxPonID)
	}
}
