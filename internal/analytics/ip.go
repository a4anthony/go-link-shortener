package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
)

// AnonymizeIP truncates an IP for privacy: it zeroes the final octet of an IPv4
// address and the final 80 bits (last 10 bytes) of an IPv6 address, mirroring
// the common GDPR "IP masking" approach. It returns nil for an unparseable IP.
func AnonymizeIP(raw string) net.IP {
	ip := net.ParseIP(raw)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		masked := make(net.IP, net.IPv4len)
		copy(masked, v4)
		masked[3] = 0
		return masked
	}
	v6 := ip.To16()
	if v6 == nil {
		return nil
	}
	masked := make(net.IP, net.IPv6len)
	copy(masked, v6)
	for i := 6; i < net.IPv6len; i++ {
		masked[i] = 0
	}
	return masked
}

// HashIP returns a salted SHA-256 hex digest of the truncated client IP. The raw
// IP is never stored; the salt makes the hash non-reversible across deployments.
// An unparseable IP yields "".
func HashIP(raw, salt string) string {
	masked := AnonymizeIP(raw)
	if masked == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write(masked)
	return hex.EncodeToString(h.Sum(nil))
}
