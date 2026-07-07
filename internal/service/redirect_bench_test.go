package service

import (
	"context"
	"testing"
)

// BenchmarkResolve_CacheHit measures the redirect hot path when the code is
// resolved from the (in-memory fake) cache — no database round trip. This
// isolates the service-layer overhead of the fast path.
func BenchmarkResolve_CacheHit(b *testing.B) {
	link := activeLink()
	svc := NewRedirectService(
		&fakeReader{},
		&fakeCache{link: link, found: true},
		nil,
		testLogger(),
	)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Resolve(ctx, "abc1234"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolve_CacheMiss measures the path where the cache misses and the
// service falls back to the (in-memory fake) database, then backfills the cache.
func BenchmarkResolve_CacheMiss(b *testing.B) {
	link := activeLink()
	svc := NewRedirectService(
		&fakeReader{link: link},
		&fakeCache{found: false},
		nil,
		testLogger(),
	)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Resolve(ctx, "abc1234"); err != nil {
			b.Fatal(err)
		}
	}
}
