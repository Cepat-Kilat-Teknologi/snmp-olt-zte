package utils

import (
	"testing"
)

func FuzzExtractONUID(f *testing.F) {
	// Seed corpus: realistic OID strings from ZTE OLTs
	f.Add(".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285278465.1")
	f.Add(".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285278465.128")
	f.Add("1.3.6.1.4.1.3902")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("no-dots")
	f.Add(".1.2.3.notanumber")
	f.Add(".....")
	f.Add(".1")
	f.Add("1")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, oid string) {
		result := ExtractONUID(oid)
		// Must never panic. Returns "" for invalid OIDs.
		_ = result
	})
}

func FuzzExtractIDOnuID(f *testing.F) {
	// Test with string inputs (the function accepts any)
	f.Add(".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285278465.1")
	f.Add("simple.42")
	f.Add("")
	f.Add("no-dots")
	f.Add(".1.2.3.notanumber")

	f.Fuzz(func(t *testing.T, oid string) {
		result := ExtractIDOnuID(oid)
		// Must never panic. Returns 0 for invalid input.
		if result < 0 {
			t.Errorf("ExtractIDOnuID(%q) returned negative: %d", oid, result)
		}
	})
}

func FuzzExtractName(f *testing.F) {
	f.Add("ONU-Customer-Name")
	f.Add("")
	f.Add("\x00\xff binary data")
	f.Add("名前テスト")     // unicode
	f.Add("a]very long name that might cause issues if not handled properly")

	f.Fuzz(func(t *testing.T, input string) {
		// Test with string type
		result := ExtractName(input)
		if result != input {
			t.Errorf("ExtractName(string %q) = %q, want %q", input, result, input)
		}

		// Test with byte slice type
		resultBytes := ExtractName([]byte(input))
		if resultBytes != input {
			t.Errorf("ExtractName([]byte %q) = %q, want %q", input, resultBytes, input)
		}
	})
}

func FuzzExtractSerialNumber(f *testing.F) {
	f.Add("ZTEGC1234567")
	f.Add("1,ZTEGC1234567")  // with "1," prefix
	f.Add("")
	f.Add("1,")               // just the prefix
	f.Add("2,SOMETHING")      // different prefix (not stripped)
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, input string) {
		// Test with string type
		result := ExtractSerialNumber(input)

		// Invariant: "1," prefix must be stripped
		if len(input) >= 2 && input[:2] == "1," {
			expected := input[2:]
			if result != expected {
				t.Errorf("ExtractSerialNumber(%q) = %q, want %q (1, prefix not stripped)", input, result, expected)
			}
		}

		// Test with byte slice type
		resultBytes := ExtractSerialNumber([]byte(input))
		if result != resultBytes {
			t.Errorf("ExtractSerialNumber string vs bytes mismatch: %q vs %q for input %q", result, resultBytes, input)
		}
	})
}

func FuzzExtractAndGetStatus(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(4)
	f.Add(7)
	f.Add(-1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, status int) {
		result := ExtractAndGetStatus(status)
		// Must never panic. Always returns a non-empty string.
		if result == "" {
			t.Errorf("ExtractAndGetStatus(%d) returned empty string", status)
		}
	})
}

func FuzzExtractLastOfflineReason(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(9)
	f.Add(13)
	f.Add(-1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, reason int) {
		result := ExtractLastOfflineReason(reason)
		// Must never panic. Always returns a non-empty string.
		if result == "" {
			t.Errorf("ExtractLastOfflineReason(%d) returned empty string", reason)
		}
	})
}
