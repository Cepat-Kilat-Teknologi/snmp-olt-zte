package usecase

import "testing"

// TestCacheKeyNamespacing verifies per-OLT Redis key namespacing: the default
// OLT (empty id) keeps legacy unprefixed keys, while a named OLT is prefixed so
// multiple OLTs sharing one Redis never collide.
func TestCacheKeyNamespacing(t *testing.T) {
	def := &onuUsecase{oltID: ""}
	if got := def.cacheKey("board_1_pon_1"); got != "board_1_pon_1" {
		t.Errorf("default OLT must not prefix: got %q", got)
	}

	ns := &onuUsecase{oltID: "c300a"}
	if got := ns.cacheKey("board_3_pon_1"); got != "olt_c300a_board_3_pon_1" {
		t.Errorf("namespaced key = %q, want olt_c300a_board_3_pon_1", got)
	}
}
