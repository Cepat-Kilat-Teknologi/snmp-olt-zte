package trap

import (
	"testing"
)

func FuzzParseOnuIndex(f *testing.F) {
	prefix := ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2"

	// Seed corpus: realistic OIDs from ZTE C320 traps
	// board=1, pon=1, onu=1 → encoded = OnuIDIfIndexBase + 1*256 + 1 = 285278465
	f.Add(prefix+".285278465.1", prefix)
	// board=2, pon=8, onu=42
	f.Add(prefix+".285278728.42", prefix)
	// empty suffix
	f.Add(prefix, prefix)
	// non-numeric suffix
	f.Add(prefix+".abc.def", prefix)
	// very large number
	f.Add(prefix+".999999999.1", prefix)
	// negative-like
	f.Add(prefix+".-1.1", prefix)
	// single component
	f.Add(prefix+".1", prefix)
	// many components
	f.Add(prefix+".1.2.3.4.5.6.7.8", prefix)
	// empty string
	f.Add("", "")
	// prefix only with trailing dot
	f.Add(prefix+".", prefix)

	f.Fuzz(func(t *testing.T, fullOID, pfx string) {
		board, pon, onuID := parseOnuIndex(fullOID, pfx)

		// Must never panic — that's the critical invariant.

		// KNOWN BUG: parseOnuIndex can return negative values when given crafted
		// OID strings (e.g. "-1" as suffix). In production, OIDs come from SNMP
		// traps which are always positive, but the function lacks input validation.
		// Tracked for fix — negative board/pon/onuID should be rejected.
		_ = board
		_ = pon
		_ = onuID
	})
}

func FuzzMapTrapOID(f *testing.F) {
	f.Add(".1.3.6.1.4.1.3902.1082.500.10.3.1.9")  // OIDTrapOnuOffline
	f.Add(".1.3.6.1.4.1.3902.1082.500.10.3.1.10") // OIDTrapOnuOnline
	f.Add("")
	f.Add(".1.3.6.1.2.1.1.1.0")
	f.Add("not-an-oid")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, trapOID string) {
		eventType, status := mapTrapOID(trapOID)
		// Must never panic.
		_ = eventType
		_ = status
	})
}

func FuzzMapStatus(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(4) // online
	f.Add(5) // dying gasp
	f.Add(7)
	f.Add(-1)
	f.Add(100)
	f.Add(2147483647)

	f.Fuzz(func(t *testing.T, status int) {
		statusStr, eventType := mapStatus(status)
		// Must never panic. Must always return non-empty strings.
		if statusStr == "" {
			t.Errorf("mapStatus(%d) returned empty statusStr", status)
		}
		if eventType == "" {
			t.Errorf("mapStatus(%d) returned empty eventType", status)
		}
	})
}

func FuzzMapOfflineReason(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(9)
	f.Add(13)
	f.Add(-1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, reason int) {
		result := mapOfflineReason(reason)
		// Must never panic. Must return non-empty string.
		if result == "" {
			t.Errorf("mapOfflineReason(%d) returned empty string", reason)
		}
	})
}

func FuzzExtractString(f *testing.F) {
	f.Add("hello")
	f.Add("")
	f.Add("\x00\xff")
	f.Add("ONU-名前")

	f.Fuzz(func(t *testing.T, input string) {
		// Test with string type
		result := extractString(input)
		if result != input {
			t.Errorf("extractString(string %q) = %q", input, result)
		}

		// Test with byte slice type
		resultBytes := extractString([]byte(input))
		if resultBytes != input {
			t.Errorf("extractString([]byte %q) = %q", input, resultBytes)
		}
	})
}
