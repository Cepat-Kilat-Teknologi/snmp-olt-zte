package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
)

// auditTrusted is the trusted set the audit log uses, installed once at
// startup by SetTrustedProxies. Unset trusts nobody.
var auditTrusted atomic.Pointer[TrustedProxies]

// SetTrustedProxies installs the peers whose X-Forwarded-For / X-Real-IP the
// audit log honors (RATE_LIMIT_TRUSTED_PROXIES).
func SetTrustedProxies(t *TrustedProxies) {
	auditTrusted.Store(t)
}

// TrustedProxies is the set of peers whose X-Forwarded-For and X-Real-IP are
// believed. A forwarded address from any other peer was written by the
// caller, so it is ignored. The zero value and nil trust nobody.
type TrustedProxies struct {
	prefixes []netip.Prefix
}

// ParseTrustedProxies reads a comma-separated list of IPs and CIDRs. Empty
// input trusts nobody. A malformed entry is an error so a typo fails startup
// instead of silently trusting more or less than intended.
func ParseTrustedProxies(raw string) (*TrustedProxies, error) {
	t := &TrustedProxies{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			p, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", entry, err)
			}
			t.prefixes = append(t.prefixes, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", entry, err)
		}
		a = a.Unmap()
		t.prefixes = append(t.prefixes, netip.PrefixFrom(a, a.BitLen()))
	}
	return t, nil
}

// Contains reports whether addr is a trusted proxy.
func (t *TrustedProxies) Contains(addr netip.Addr) bool {
	if t == nil || !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, p := range t.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ClientIP returns the client address for r. See ResolveClientIP.
func (t *TrustedProxies) ClientIP(r *http.Request) string {
	return t.ResolveClientIP(r.RemoteAddr, r.Header.Values("X-Forwarded-For"), r.Header.Get("X-Real-IP"))
}

// ResolveClientIP picks the client address from the TCP peer and the
// forwarding headers. The headers are read only when the peer is trusted.
// X-Forwarded-For lines are joined and walked right to left: the first hop
// that is not itself a trusted proxy is the client, because everything left
// of it was written by that client. A malformed hop stops the walk and the
// peer is used. X-Real-IP is consulted only when there is no X-Forwarded-For.
func (t *TrustedProxies) ResolveClientIP(remoteAddr string, xff []string, xRealIP string) string {
	peer := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		peer = host
	}
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	peer = peerAddr.Unmap().String()
	if !t.Contains(peerAddr) {
		return peer
	}

	if chain := strings.Join(xff, ","); strings.TrimSpace(chain) != "" {
		hops := strings.Split(chain, ",")
		client := peer
		for i := len(hops) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil {
				return peer
			}
			client = a.Unmap().String()
			if !t.Contains(a) {
				return client
			}
		}
		// Every hop is a trusted proxy: the leftmost one is the origin.
		return client
	}

	if xRealIP = strings.TrimSpace(xRealIP); xRealIP != "" {
		if a, err := netip.ParseAddr(xRealIP); err == nil {
			return a.Unmap().String()
		}
	}
	return peer
}
