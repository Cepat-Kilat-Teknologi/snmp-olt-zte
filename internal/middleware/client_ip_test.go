package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTrusted(t *testing.T, raw string) *TrustedProxies {
	t.Helper()
	tp, err := ParseTrustedProxies(raw)
	require.NoError(t, err)
	return tp
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"empty", "", false},
		{"only separators", " , ,", false},
		{"single ip", "10.42.0.5", false},
		{"cidr and ip with spaces", " 10.42.0.0/16 , 192.168.1.1 ", false},
		{"ipv6", "fd00::/8,::1", false},
		{"bad ip", "10.42.0.256", true},
		{"bad cidr prefix", "10.42.0.0/33", true},
		{"hostname", "ingress-nginx", true},
		{"one bad entry fails all", "10.0.0.1,nope", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTrustedProxies(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTrustedProxies_Contains(t *testing.T) {
	tp := mustTrusted(t, "10.42.0.0/16,192.168.1.1")
	tests := []struct {
		name string
		tp   *TrustedProxies
		addr netip.Addr
		want bool
	}{
		{"in cidr", tp, netip.MustParseAddr("10.42.9.9"), true},
		{"exact ip", tp, netip.MustParseAddr("192.168.1.1"), true},
		{"ipv4-mapped ipv6", tp, netip.MustParseAddr("::ffff:10.42.1.1"), true},
		{"outside", tp, netip.MustParseAddr("10.43.0.1"), false},
		{"invalid addr", tp, netip.Addr{}, false},
		{"nil set", nil, netip.MustParseAddr("10.42.0.1"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tp.Contains(tt.addr))
		})
	}
}

func TestTrustedProxies_ClientIP(t *testing.T) {
	trusted := mustTrusted(t, "10.42.0.0/16")
	tests := []struct {
		name   string
		tp     *TrustedProxies
		remote string
		xff    []string
		realIP string
		want   string
	}{
		{"no headers", trusted, "203.0.113.9:4000", nil, "", "203.0.113.9"},
		{"spoofed xff from untrusted peer ignored", trusted, "203.0.113.9:4000", []string{"1.2.3.4"}, "", "203.0.113.9"},
		{"spoofed x-real-ip from untrusted peer ignored", trusted, "203.0.113.9:4000", nil, "1.2.3.4", "203.0.113.9"},
		{"empty config trusts nobody", mustTrusted(t, ""), "10.42.1.7:80", []string{"1.2.3.4"}, "", "10.42.1.7"},
		{"nil config trusts nobody", nil, "10.42.1.7:80", []string{"1.2.3.4"}, "1.2.3.4", "10.42.1.7"},
		{"trusted peer single hop", trusted, "10.42.1.7:80", []string{"198.51.100.1"}, "", "198.51.100.1"},
		{"trusted peer leftmost spoof ignored", trusted, "10.42.1.7:80", []string{"6.6.6.6, 198.51.100.1, 10.42.3.3"}, "", "198.51.100.1"},
		{"multiple xff lines joined", trusted, "10.42.1.7:80", []string{"6.6.6.6, 198.51.100.1", "10.42.3.3"}, "", "198.51.100.1"},
		{"all hops trusted uses leftmost", trusted, "10.42.1.7:80", []string{"10.42.0.1, 10.42.0.2"}, "", "10.42.0.1"},
		{"malformed hop falls back to peer", trusted, "10.42.1.7:80", []string{"198.51.100.1, garbage"}, "", "10.42.1.7"},
		{"empty hop falls back to peer", trusted, "10.42.1.7:80", []string{"198.51.100.1,"}, "", "10.42.1.7"},
		{"whitespace xff treated as absent", trusted, "10.42.1.7:80", []string{"   "}, "", "10.42.1.7"},
		{"x-real-ip from trusted peer", trusted, "10.42.1.7:80", nil, "198.51.100.1", "198.51.100.1"},
		{"xff wins over x-real-ip", trusted, "10.42.1.7:80", []string{"198.51.100.1"}, "6.6.6.6", "198.51.100.1"},
		{"malformed x-real-ip falls back to peer", trusted, "10.42.1.7:80", nil, "nope", "10.42.1.7"},
		{"ipv6 hop", trusted, "10.42.1.7:80", []string{"2001:db8::1"}, "", "2001:db8::1"},
		{"mapped peer is unmapped", nil, "[::ffff:203.0.113.9]:80", nil, "", "203.0.113.9"},
		{"remote addr without port", trusted, "192.168.1.1", nil, "", "192.168.1.1"},
		{"unparseable remote addr returned as is", trusted, "pipe", []string{"1.2.3.4"}, "", "pipe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			assert.Equal(t, tt.want, tt.tp.ClientIP(req))
		})
	}
}
