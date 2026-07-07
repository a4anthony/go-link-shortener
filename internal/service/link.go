package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/shortcode"
)

// LinkRepository is the persistence LinkService depends on. The concrete
// repository.LinkRepository satisfies it structurally.
type LinkRepository interface {
	Create(ctx context.Context, l *domain.Link) error
	GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Link, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.Link, error)
	Update(ctx context.Context, l *domain.Link) error
	SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error
	ExistsByCode(ctx context.Context, code string) (bool, error)
}

// CodeGenerator produces candidate short codes.
type CodeGenerator interface {
	Generate() (string, error)
}

// LinkEventSink is notified when links change so webhooks can be dispatched. It
// is optional; a nil sink disables events.
type LinkEventSink interface {
	LinkCreated(l *domain.Link)
}

// LinkService implements link CRUD and shortcode allocation.
type LinkService struct {
	repo         LinkRepository
	gen          CodeGenerator
	events       LinkEventSink
	maxCollision int
	now          func() time.Time
}

// NewLinkService builds a LinkService. maxCollision bounds shortcode
// regeneration attempts. events may be nil.
func NewLinkService(repo LinkRepository, gen CodeGenerator, events LinkEventSink, maxCollision int) *LinkService {
	if maxCollision < 1 {
		maxCollision = 1
	}
	return &LinkService{
		repo:         repo,
		gen:          gen,
		events:       events,
		maxCollision: maxCollision,
		now:          time.Now,
	}
}

// CreateLinkInput is the validated input for creating a link.
type CreateLinkInput struct {
	TargetURL    string
	CustomAlias  string // empty => auto-generate
	RedirectType int    // 0 => default 302
	ExpiresAt    *time.Time
	MaxClicks    *int64
}

// UpdateLinkInput carries optional fields to patch. Nil pointers are left
// unchanged; a non-nil pointer to the zero value explicitly clears (for
// ExpiresAt/MaxClicks) via the Clear* flags.
type UpdateLinkInput struct {
	TargetURL      *string
	RedirectType   *int
	ExpiresAt      *time.Time
	ClearExpiresAt bool
	MaxClicks      *int64
	ClearMaxClicks bool
}

// Create validates input, allocates a code (custom alias or generated), and
// persists the link.
func (s *LinkService) Create(ctx context.Context, tenantID uuid.UUID, in CreateLinkInput) (*domain.Link, error) {
	if err := validateURL(in.TargetURL); err != nil {
		return nil, err
	}
	redirectType, err := normalizeRedirectType(in.RedirectType)
	if err != nil {
		return nil, err
	}
	if err := s.validateExpiry(in.ExpiresAt); err != nil {
		return nil, err
	}
	if err := validateMaxClicks(in.MaxClicks); err != nil {
		return nil, err
	}

	link := &domain.Link{
		TenantID:     tenantID,
		TargetURL:    in.TargetURL,
		RedirectType: redirectType,
		ExpiresAt:    in.ExpiresAt,
		MaxClicks:    in.MaxClicks,
	}

	if in.CustomAlias != "" {
		if !shortcode.ValidAlias(in.CustomAlias) {
			return nil, fmt.Errorf("%w: custom alias must be 3-64 URL-safe characters and not reserved", domain.ErrValidation)
		}
		link.Code = in.CustomAlias
		if err := s.repo.Create(ctx, link); err != nil {
			return nil, err // ErrConflict surfaces a taken alias
		}
	} else {
		if err := s.allocateGeneratedCode(ctx, link); err != nil {
			return nil, err
		}
	}

	s.emitCreated(link)
	return link, nil
}

// allocateGeneratedCode generates codes and inserts, retrying on collision up to
// the configured budget.
func (s *LinkService) allocateGeneratedCode(ctx context.Context, link *domain.Link) error {
	for attempt := 0; attempt < s.maxCollision; attempt++ {
		code, err := s.gen.Generate()
		if err != nil {
			return fmt.Errorf("generate shortcode: %w", err)
		}
		link.Code = code
		err = s.repo.Create(ctx, link)
		if err == nil {
			return nil
		}
		if errors.Is(err, domain.ErrConflict) {
			continue // collision; try a fresh code
		}
		return err
	}
	return domain.ErrCodeExhausted
}

// Get returns a tenant's link by id.
func (s *LinkService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Link, error) {
	return s.repo.GetByIDForTenant(ctx, tenantID, id)
}

// List returns a page of a tenant's links. limit is clamped to [1,100].
func (s *LinkService) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.Link, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListByTenant(ctx, tenantID, limit, offset)
}

// Update applies a partial update to a tenant's link.
func (s *LinkService) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateLinkInput) (*domain.Link, error) {
	link, err := s.repo.GetByIDForTenant(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if in.TargetURL != nil {
		if err := validateURL(*in.TargetURL); err != nil {
			return nil, err
		}
		link.TargetURL = *in.TargetURL
	}
	if in.RedirectType != nil {
		rt, err := normalizeRedirectType(*in.RedirectType)
		if err != nil {
			return nil, err
		}
		link.RedirectType = rt
	}
	switch {
	case in.ClearExpiresAt:
		link.ExpiresAt = nil
	case in.ExpiresAt != nil:
		if err := s.validateExpiry(in.ExpiresAt); err != nil {
			return nil, err
		}
		link.ExpiresAt = in.ExpiresAt
	}
	switch {
	case in.ClearMaxClicks:
		link.MaxClicks = nil
	case in.MaxClicks != nil:
		if err := validateMaxClicks(in.MaxClicks); err != nil {
			return nil, err
		}
		link.MaxClicks = in.MaxClicks
	}

	if err := s.repo.Update(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// Delete soft-deletes a tenant's link.
func (s *LinkService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, tenantID, id)
}

func (s *LinkService) emitCreated(l *domain.Link) {
	if s.events != nil {
		s.events.LinkCreated(l)
	}
}

func (s *LinkService) validateExpiry(t *time.Time) error {
	if t != nil && !t.After(s.now()) {
		return fmt.Errorf("%w: expires_at must be in the future", domain.ErrValidation)
	}
	return nil
}

func validateURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: url is required", domain.ErrValidation)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: url is not parseable", domain.ErrValidation)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: url scheme must be http or https", domain.ErrValidation)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: url must include a host", domain.ErrValidation)
	}
	return nil
}

func normalizeRedirectType(rt int) (int, error) {
	switch rt {
	case 0, domain.RedirectFound:
		return domain.RedirectFound, nil
	case domain.RedirectPermanent:
		return domain.RedirectPermanent, nil
	default:
		return 0, fmt.Errorf("%w: redirect_type must be 301 or 302", domain.ErrValidation)
	}
}

func validateMaxClicks(m *int64) error {
	if m != nil && *m < 1 {
		return fmt.Errorf("%w: max_clicks must be a positive integer", domain.ErrValidation)
	}
	return nil
}
