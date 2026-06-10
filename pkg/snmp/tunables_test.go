package snmp

import (
	"testing"
	"time"
)

func TestSnmpTimeout(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		val  string
		want time.Duration
	}{
		{"unset default", false, "", 5 * time.Second},
		{"valid override", true, "12", 12 * time.Second},
		{"non-numeric falls back", true, "abc", 5 * time.Second},
		{"zero falls back", true, "0", 5 * time.Second},
		{"negative falls back", true, "-3", 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("SNMP_TIMEOUT_SECONDS", tt.val)
			} else {
				t.Setenv("SNMP_TIMEOUT_SECONDS", "")
			}
			if got := snmpTimeout(); got != tt.want {
				t.Errorf("snmpTimeout()=%v want %v", got, tt.want)
			}
		})
	}
}

func TestSnmpRetries(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset default", false, "", 2},
		{"valid override", true, "7", 7},
		{"zero allowed", true, "0", 0},
		{"non-numeric falls back", true, "abc", 2},
		{"negative falls back", true, "-1", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("SNMP_RETRIES", tt.val)
			} else {
				t.Setenv("SNMP_RETRIES", "")
			}
			if got := snmpRetries(); got != tt.want {
				t.Errorf("snmpRetries()=%d want %d", got, tt.want)
			}
		})
	}
}
