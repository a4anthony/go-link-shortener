package analytics

import "net"

// GeoResolver maps a client IP to an ISO 3166-1 alpha-2 country code. This is
// the seam for IP geolocation: the default is a no-op, and a MaxMind-backed
// resolver can be dropped in without touching the pipeline.
type GeoResolver interface {
	// Country returns the ISO country code for ip, or "" if unknown. It must not
	// return an error for a simply-unresolvable IP — only for backend failures.
	Country(ip net.IP) (string, error)
}

// NoopResolver resolves every IP to an empty country. It is the default so the
// service runs with no geo database configured.
type NoopResolver struct{}

// Country always returns "".
func (NoopResolver) Country(net.IP) (string, error) { return "", nil }

// ResolverFunc adapts a function to the GeoResolver interface.
type ResolverFunc func(net.IP) (string, error)

// Country calls the wrapped function.
func (f ResolverFunc) Country(ip net.IP) (string, error) { return f(ip) }

// NewMaxMindResolver adapts a MaxMind geoip2 reader to GeoResolver. It takes a
// lookup with the same shape as (*geoip2.Reader).Country reduced to the ISO code,
// so wiring MaxMind is a one-liner and this package keeps no dependency on it:
//
//	reader, _ := geoip2.Open("GeoLite2-Country.mmdb")
//	resolver := analytics.NewMaxMindResolver(func(ip net.IP) (string, error) {
//	    rec, err := reader.Country(ip)
//	    if err != nil {
//	        return "", err
//	    }
//	    return rec.Country.IsoCode, nil
//	})
func NewMaxMindResolver(lookup func(net.IP) (string, error)) GeoResolver {
	return ResolverFunc(lookup)
}
