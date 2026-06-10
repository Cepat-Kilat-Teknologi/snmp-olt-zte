package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
)

// okUsecase wraps the base mock but returns a non-empty ONU list so authorized
// board/pon requests return 200 (used to distinguish authz blocks from handler 404s).
type okUsecase struct{ *mockOnuUsecase }

func (okUsecase) GetByBoardIDAndPonID(_ context.Context, _, _ int) ([]model.ONUInfoPerBoard, error) {
	return []model.ONUInfoPerBoard{{}}, nil
}

// TestLoadRoutesMulti_PerOLTValidation proves that one instance serves multiple
// OLTs with per-OLT slot sets AND per-slot PON validation: a C320 (slots 1,2 x16)
// and a C300 (slot 3 GTGH=16, slot 5 GTGO=8) side by side, plus the default OLT
// served on the bare /board paths.
func TestLoadRoutesMulti_PerOLTValidation(t *testing.T) {
	t.Setenv("API_KEY", "") // disable API key auth for the test

	h := handler.NewOnuHandler(&mockOnuUsecase{})
	olts := []oltRoute{
		{id: "c320", handler: h, boardPons: map[int]int{1: 16, 2: 16}},
		{id: "c300a", handler: h, boardPons: map[int]int{3: 16, 5: 8}},
	}
	router := loadRoutesMulti(olts, "c320", nil, nil, "")

	// want400 = request should be rejected by board/pon validation (400).
	// !want400 = validation passes and the request reaches the handler.
	cases := []struct {
		path    string
		want400 bool
	}{
		{"/api/v1/olt/c320/board/1/pon/1", false},   // valid C320 slot
		{"/api/v1/olt/c320/board/3/pon/1", true},    // slot 3 not on C320
		{"/api/v1/olt/c300a/board/3/pon/16", false}, // slot 3 is GTGH (16)
		{"/api/v1/olt/c300a/board/5/pon/8", false},  // slot 5 is GTGO (8)
		{"/api/v1/olt/c300a/board/5/pon/9", true},   // slot 5 GTGO has only 8 PONs
		{"/api/v1/olt/c300a/board/1/pon/1", true},   // slot 1 not on this C300
		{"/api/v1/board/1/pon/1", false},            // bare path -> default OLT (c320)
		{"/api/v1/board/3/pon/1", true},             // bare default c320: slot 3 invalid
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		got400 := rr.Code == http.StatusBadRequest
		if got400 != tc.want400 {
			t.Errorf("%s: status=%d want400=%v", tc.path, rr.Code, tc.want400)
		}
	}
}

// TestLoadRoutesMulti_UnknownOLT verifies a request to an unconfigured OLT id
// does not match any route (404).
func TestLoadRoutesMulti_UnknownOLT(t *testing.T) {
	t.Setenv("API_KEY", "")
	h := handler.NewOnuHandler(&mockOnuUsecase{})
	router := loadRoutesMulti([]oltRoute{{id: "c320", handler: h, boardPons: map[int]int{1: 16}}}, "c320", nil, nil, "")

	req := httptest.NewRequest("GET", "/api/v1/olt/nope/board/1/pon/1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown OLT: status=%d, want 404", rr.Code)
	}
}

// TestLoadRoutesMulti_TenantIsolation proves per-user scoping: with an API_USERS
// registry, user 1 sees only its OLT, user 2 only its own, admin sees both, a
// bad key is 401, and a cross-tenant request is 404 (not 403 — no enumeration).
func TestLoadRoutesMulti_TenantIsolation(t *testing.T) {
	// okUsecase returns a non-empty ONU list so an authorized request yields 200,
	// making "owner reaches handler (200)" distinguishable from "blocked by
	// ownership (404)". (The base mock returns nil -> the handler 404s on empty.)
	h := handler.NewOnuHandler(okUsecase{&mockOnuUsecase{}})
	olts := []oltRoute{
		{id: "c320", userID: 1, handler: h, boardPons: map[int]int{1: 16}},
		{id: "c300a", userID: 2, handler: h, boardPons: map[int]int{3: 16}},
	}
	users := map[string]reqctx.Principal{
		"keyA":     {UserID: 1},
		"keyB":     {UserID: 2},
		"adminKey": {Admin: true},
	}
	router := loadRoutesMulti(olts, "c320", nil, users, "")

	cases := []struct {
		name string
		key  string
		path string
		want int
	}{
		{"user1 own OLT", "keyA", "/api/v1/olt/c320/board/1/pon/1", http.StatusOK},
		{"user1 other OLT -> 404", "keyA", "/api/v1/olt/c300a/board/3/pon/1", http.StatusNotFound},
		{"user2 own OLT", "keyB", "/api/v1/olt/c300a/board/3/pon/1", http.StatusOK},
		{"user2 other OLT -> 404", "keyB", "/api/v1/olt/c320/board/1/pon/1", http.StatusNotFound},
		{"admin sees c320", "adminKey", "/api/v1/olt/c320/board/1/pon/1", http.StatusOK},
		{"admin sees c300a", "adminKey", "/api/v1/olt/c300a/board/3/pon/1", http.StatusOK},
		{"missing key -> 401", "", "/api/v1/olt/c320/board/1/pon/1", http.StatusUnauthorized},
		{"bad key -> 401", "nope", "/api/v1/olt/c320/board/1/pon/1", http.StatusUnauthorized},
		{"user2 bare default(c320) -> 404", "keyB", "/api/v1/board/1/pon/1", http.StatusNotFound},
		{"user1 bare default(c320) -> 200", "keyA", "/api/v1/board/1/pon/1", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			if tc.key != "" {
				req.Header.Set("X-API-Key", tc.key)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Errorf("%s %s: status=%d, want %d", tc.key, tc.path, rr.Code, tc.want)
			}
		})
	}
}
