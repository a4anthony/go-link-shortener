package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeReader struct {
	link *domain.Link
	err  error
	hits int
}

func (f *fakeReader) GetByCode(_ context.Context, _ string) (*domain.Link, error) {
	f.hits++
	return f.link, f.err
}

type fakeCache struct {
	link     *domain.Link
	found    bool
	getErr   error
	setCalls int
	negCalls int
	invCalls int
}

func (f *fakeCache) Get(_ context.Context, _ string) (*domain.Link, bool, error) {
	return f.link, f.found, f.getErr
}
func (f *fakeCache) Set(_ context.Context, _ *domain.Link) error { f.setCalls++; return nil }
func (f *fakeCache) SetNegative(_ context.Context, _ string) error { f.negCalls++; return nil }
func (f *fakeCache) Invalidate(_ context.Context, _ string) error  { f.invCalls++; return nil }

type countingObserver struct{ hits, misses int }

func (o *countingObserver) ObserveCacheHit()  { o.hits++ }
func (o *countingObserver) ObserveCacheMiss() { o.misses++ }

func activeLink() *domain.Link {
	return &domain.Link{ID: uuid.New(), TenantID: uuid.New(), Code: "abc1234", TargetURL: "https://example.com", RedirectType: 302}
}

func TestRedirect_CacheHit(t *testing.T) {
	link := activeLink()
	cache := &fakeCache{link: link, found: true}
	db := &fakeReader{}
	obs := &countingObserver{}
	svc := NewRedirectService(db, cache, obs, testLogger())

	got, err := svc.Resolve(context.Background(), "abc1234")
	require.NoError(t, err)
	assert.Equal(t, link.TargetURL, got.TargetURL)
	assert.Equal(t, 0, db.hits, "cache hit must not touch db")
	assert.Equal(t, 1, obs.hits)
}

func TestRedirect_NegativeCacheHit(t *testing.T) {
	cache := &fakeCache{link: nil, found: true}
	db := &fakeReader{}
	svc := NewRedirectService(db, cache, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, 0, db.hits, "negative cache must not touch db")
}

func TestRedirect_MissBackfills(t *testing.T) {
	link := activeLink()
	cache := &fakeCache{found: false}
	db := &fakeReader{link: link}
	obs := &countingObserver{}
	svc := NewRedirectService(db, cache, obs, testLogger())

	got, err := svc.Resolve(context.Background(), "abc1234")
	require.NoError(t, err)
	assert.Equal(t, link.TargetURL, got.TargetURL)
	assert.Equal(t, 1, db.hits)
	assert.Equal(t, 1, cache.setCalls, "db fallback should backfill the cache")
	assert.Equal(t, 1, obs.misses)
}

func TestRedirect_MissNotFoundCachesNegative(t *testing.T) {
	cache := &fakeCache{found: false}
	db := &fakeReader{err: domain.ErrNotFound}
	svc := NewRedirectService(db, cache, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "nope")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, 1, cache.negCalls, "missing code should be negatively cached")
}

func TestRedirect_ExpiredIsGone(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	link := activeLink()
	link.ExpiresAt = &past
	cache := &fakeCache{link: link, found: true}
	svc := NewRedirectService(&fakeReader{}, cache, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "abc1234")
	assert.ErrorIs(t, err, domain.ErrGone)
}

func TestRedirect_ExhaustedIsGone(t *testing.T) {
	max := int64(10)
	link := activeLink()
	link.MaxClicks = &max
	link.ClickCount = 10
	svc := NewRedirectService(&fakeReader{link: link}, &fakeCache{found: false}, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "abc1234")
	assert.ErrorIs(t, err, domain.ErrGone)
}

func TestRedirect_CacheErrorFallsBackToDB(t *testing.T) {
	link := activeLink()
	cache := &fakeCache{getErr: errors.New("redis down")}
	db := &fakeReader{link: link}
	obs := &countingObserver{}
	svc := NewRedirectService(db, cache, obs, testLogger())

	got, err := svc.Resolve(context.Background(), "abc1234")
	require.NoError(t, err)
	assert.Equal(t, link.TargetURL, got.TargetURL)
	assert.Equal(t, 1, db.hits, "cache error should fail open to db")
	assert.Equal(t, 1, obs.misses)
}
