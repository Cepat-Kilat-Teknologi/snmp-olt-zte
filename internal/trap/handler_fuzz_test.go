package trap

import (
	"testing"
)

func FuzzResolveEventType(f *testing.F) {
	// Seed corpus: realistic status and offline reason strings from SNMP
	f.Add("Online", "")
	f.Add("online", "")
	f.Add("Offline", "")
	f.Add("offline", "LOS")
	f.Add("offline", "LOSi")
	f.Add("offline", "LOFi")
	f.Add("offline", "DyingGasp")
	f.Add("offline", "PowerOff")
	f.Add("offline", "AuthFail")
	f.Add("Dying Gasp", "")
	f.Add("LOS", "")
	f.Add("PowerOff", "")
	f.Add("AuthFailed", "")
	f.Add("Logging", "")
	f.Add("Synchronization", "")
	f.Add("", "")
	f.Add("unknown status", "")
	f.Add("\x00\xff", "")
	f.Add("ONLINE", "")  // case sensitivity
	f.Add("partially_online", "")

	f.Fuzz(func(t *testing.T, status, offlineReason string) {
		result := resolveEventType(status, offlineReason)

		// Must never panic. Must return non-empty string.
		if result == "" {
			t.Errorf("resolveEventType(%q, %q) returned empty string", status, offlineReason)
		}
	})
}

func FuzzNormalizeOfflineReason(f *testing.F) {
	f.Add("LOS")
	f.Add("LOSi")
	f.Add("LOFi")
	f.Add("DyingGasp")
	f.Add("PowerOff")
	f.Add("AuthFail")
	f.Add("unknown")
	f.Add("")
	f.Add("\x00\xff")
	f.Add("los mixed case")
	f.Add("dying gasp with spaces")

	f.Fuzz(func(t *testing.T, reason string) {
		result := normalizeOfflineReason(reason)

		// Must never panic. Must return non-empty string.
		if result == "" {
			t.Errorf("normalizeOfflineReason(%q) returned empty string", reason)
		}
	})
}
