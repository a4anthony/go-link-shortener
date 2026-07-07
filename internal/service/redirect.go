package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// LinkReader is the minimal persistence the redirect path needs: a single
// by-code lookup. repository.LinkRepository satisfies it.
type LinkReader interface {
	GetByCode(ctx context.Context, code string) (*domain.Link, error)
}

// LinkCache is the redirect-path cache contract. Get's bool reports whether the
// cache had an authoritative answer (see repository.LinkCache.Get).
type LinkCache interface {
	Get(ctx context.Context, code string) (*domain.Link, bool, error)
	Set(ctx context.Context, l *domain.Link) error
	SetNegative(ctx context.Context, code string) error
	Invalidate(ctx context.Context, code string) error
}

// CacheObserver is notified of cache outcomes so metrics can track hit ratio.
// Optional; a nil observer disables observation.
type CacheObserver interface {
	ObserveCacheHit()
	ObserveCacheMiss()
}

// RedirectService resolves a public short code to its target on the hot path:
// Redis first, Postgres fallback with cache backfill. It never blocks on
// anything but the lookup itself.
type RedirectService struct {
	db       LinkReader
	cache    LinkCache
	observer CacheObserver
	log      *slog.Logger
	now      func() time.Time
}

// NewRedirectService builds a RedirectService. observer may be nil.
func NewRedirectService(db LinkReader, cache LinkCache, observer CacheObserver, log *slog.Logger) *RedirectService {
	return &RedirectService{
		db:       db,
		cache:    cache,
		observer: observer,
		log:      log,
		now:      time.Now,
	}
}

// Resolve returns the live link for a code, or domain.ErrNotFound / domain.ErrGone.
// It reads Redis first and backfills the cache on a Postgres fallback.
func (s *RedirectService) Resolve(ctx context.Context, code string) (*domain.Link, error) {
	link, authoritative, err := s.cache.Get(ctx, code)
	switch {
	case err != nil:
		// Cache failure: log and fall through to the database (fail-open).
		s.log.Warn("link cache read failed; falling back to db", "error", err, "code", code)
		s.observeMiss()
		return s.fromDB(ctx, code)
	case authoritative && link != nil:
		s.observeHit()
		return s.checkActive(link)
	case authoritative && link == nil:
		// Negative cache hit: known-missing code.
		s.observeHit()
		return nil, domain.ErrNotFound
	default:
		s.observeMiss()
		return s.fromDB(ctx, code)
	}
}

// fromDB loads the link from Postgres and backfills the cache.
func (s *RedirectService) fromDB(ctx context.Context, code string) (*domain.Link, error) {
	link, err := s.db.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			if cerr := s.cache.SetNegative(ctx, code); cerr != nil {
				s.log.Warn("failed to write negative cache entry", "error", cerr, "code", code)
			}
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if cerr := s.cache.Set(ctx, link); cerr != nil {
		s.log.Warn("failed to backfill link cache", "error", cerr, "code", code)
	}
	return s.checkActive(link)
}

// checkActive maps expiry/exhaustion to domain.ErrGone.
func (s *RedirectService) checkActive(link *domain.Link) (*domain.Link, error) {
	if !link.IsActive(s.now()) {
		return nil, domain.ErrGone
	}
	return link, nil
}

func (s *RedirectService) observeHit() {
	if s.observer != nil {
		s.observer.ObserveCacheHit()
	}
}

func (s *RedirectService) observeMiss() {
	if s.observer != nil {
		s.observer.ObserveCacheMiss()
	}
}
