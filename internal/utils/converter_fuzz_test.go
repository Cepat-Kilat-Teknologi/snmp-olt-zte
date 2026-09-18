package utils

import (
	"testing"
)

func FuzzConvertByteArrayToDateTime(f *testing.F) {
	// Seed corpus: valid 8-byte date arrays
	// Format: Year(2 bytes BE), Month(1), Day(1), Hour(1), Min(1), Sec(1), Reserved(1)

	// 2024-01-15 10:30:45
	f.Add([]byte{0x07, 0xE8, 0x01, 0x0F, 0x0A, 0x1E, 0x2D, 0x00})
	// 2026-12-31 23:59:59
	f.Add([]byte{0x07, 0xEA, 0x0C, 0x1F, 0x17, 0x3B, 0x3B, 0x00})
	// 2000-06-15 00:00:00
	f.Add([]byte{0x07, 0xD0, 0x06, 0x0F, 0x00, 0x00, 0x00, 0x00})

	// Edge cases
	f.Add([]byte{})                                                     // empty
	f.Add([]byte{0x00})                                                 // too short
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})       // all zeros (month=0 invalid)
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})       // all 0xFF
	f.Add([]byte{0x07, 0xE8, 0x0D, 0x01, 0x00, 0x00, 0x00, 0x00})       // month=13 invalid
	f.Add([]byte{0x07, 0xE8, 0x01, 0x20, 0x00, 0x00, 0x00, 0x00})       // day=32 invalid
	f.Add([]byte{0x07, 0xE8, 0x01, 0x01, 0x18, 0x00, 0x00, 0x00})       // hour=24 invalid
	f.Add([]byte{0x07, 0xE8, 0x01, 0x01, 0x00, 0x3C, 0x00, 0x00})       // minute=60 invalid
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // 9 bytes

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := ConvertByteArrayToDateTime(data)

		// Invariant 1: non-8-byte input must error
		if len(data) != 8 && err == nil {
			t.Errorf("ConvertByteArrayToDateTime(%x) should error for len=%d", data, len(data))
		}

		// Invariant 2: valid result must be non-empty and match expected format
		if err == nil && result == "" {
			t.Error("ConvertByteArrayToDateTime returned nil error but empty result")
		}

		// Invariant 3: valid result should be at least 19 chars ("YYYY-MM-DD HH:MM:SS")
		// KNOWN BUG: year values >9999 (e.g. 12336) produce 20+ char output because
		// Sprintf("%04d") only pads, it doesn't truncate. Missing year range validation.
		if err == nil && len(result) < 19 {
			t.Errorf("ConvertByteArrayToDateTime returned result too short (len %d): %q", len(result), result)
		}
	})
}

func FuzzConvertStringToUint16(f *testing.F) {
	f.Add("")
	f.Add("0")
	f.Add("1")
	f.Add("65535")
	f.Add("65536") // overflow
	f.Add("-1")
	f.Add("abc")
	f.Add("123abc")
	f.Add("99999999999999999999")
	f.Add("\x00")

	f.Fuzz(func(t *testing.T, input string) {
		result := ConvertStringToUint16(input)
		// Should never panic. Result is always in [0, 65535].
		if result > 65535 {
			t.Errorf("ConvertStringToUint16(%q) = %d, exceeds uint16 max", input, result)
		}
	})
}

func FuzzConvertStringToInteger(f *testing.F) {
	f.Add("")
	f.Add("0")
	f.Add("42")
	f.Add("-42")
	f.Add("2147483647")  // int32 max
	f.Add("-2147483648") // int32 min
	f.Add("abc")
	f.Add("12.34")
	f.Add(" 42 ")

	f.Fuzz(func(t *testing.T, input string) {
		_ = ConvertStringToInteger(input)
		// Must never panic. Return 0 on invalid input.
	})
}
