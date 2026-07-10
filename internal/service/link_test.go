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

// fakeLinkRepo is an in-memory LinkRepository keyed by code (live links only).
type fakeLinkRepo struct {
	byCode    map[string]*domain.Link
	createErr error
}

func newFakeLinkRepo() *fakeLinkRepo {
	return &fakeLinkRepo{byCode: map[string]*domain.Link{}}
}

func (f *fakeLinkRepo) Create(_ context.Context, l *domain.Link) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, ok := f.byCode[l.Code]; ok {
		return domain.ErrConflict
	}
	l.ID = uuid.New()
	l.CreatedAt = time.Now()
	l.UpdatedAt = l.CreatedAt
	stored := *l
	f.byCode[l.Code] = &stored
	return nil
}

func (f *fakeLinkRepo) GetByIDForTenant(_ context.Context, tenantID, id uuid.UUID) (*domain.Link, error) {
	for _, l := range f.byCode {
		if l.ID == id && l.TenantID == tenantID {
			cp := *l
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeLinkRepo) ListByTenant(_ context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.Link, error) {
	var out []domain.Link
	for _, l := range f.byCode {
		if l.TenantID == tenantID {
			out = append(out, *l)
		}
	}
	if offset >= len(out) {
		return nil, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (f *fakeLinkRepo) Update(_ context.Context, l *domain.Link) error {
	for code, existing := range f.byCode {
		if existing.ID == l.ID && existing.TenantID == l.TenantID {
			delete(f.byCode, code)
			l.UpdatedAt = time.Now()
			stored := *l
			f.byCode[l.Code] = &stored
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeLinkRepo) SoftDelete(_ context.Context, tenantID, id uuid.UUID) error {
	for code, l := range f.byCode {
		if l.ID == id && l.TenantID == tenantID {
			delete(f.byCode, code)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeLinkRepo) ExistsByCode(_ context.Context, code string) (bool, error) {
	_, ok := f.byCode[code]
	return ok, nil
}

// scriptedGen yields a fixed sequence of codes, then repeats the last.
type scriptedGen struct {
	codes []string
	i     int
}

func (g *scriptedGen) Generate() (string, error) {
	c := g.codes[g.i]
	if g.i < len(g.codes)-1 {
		g.i++
	}
	return c, nil
}

func TestLinkService_Create_Generated(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"abc1234"}}, nil, nil, 5, testLogger())

	link, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "abc1234", link.Code)
	assert.Equal(t, domain.RedirectFound, link.RedirectType)
}

func TestLinkService_Create_CustomAlias(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"x"}}, nil, nil, 5, testLogger())
	tenant := uuid.New()

	link, err := svc.Create(context.Background(), tenant, CreateLinkInput{
		TargetURL: "https://example.com", CustomAlias: "promo-2026",
	})
	require.NoError(t, err)
	assert.Equal(t, "promo-2026", link.Code)

	// Duplicate alias conflicts.
	_, err = svc.Create(context.Background(), tenant, CreateLinkInput{
		TargetURL: "https://example.com", CustomAlias: "promo-2026",
	})
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestLinkService_Create_InvalidAlias(t *testing.T) {
	svc := NewLinkService(newFakeLinkRepo(), &scriptedGen{codes: []string{"x"}}, nil, nil, 5, testLogger())
	_, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{
		TargetURL: "https://example.com", CustomAlias: "no spaces",
	})
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestLinkService_Create_CollisionRetryThenExhausted(t *testing.T) {
	repo := newFakeLinkRepo()
	// Pre-populate the colliding code so every generated attempt conflicts.
	repo.byCode["dup"] = &domain.Link{ID: uuid.New(), Code: "dup", TenantID: uuid.New()}
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"dup"}}, nil, nil, 3, testLogger())

	_, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://example.com"})
	assert.ErrorIs(t, err, domain.ErrCodeExhausted)
}

func TestLinkService_Create_CollisionRetrySucceeds(t *testing.T) {
	repo := newFakeLinkRepo()
	repo.byCode["taken"] = &domain.Link{ID: uuid.New(), Code: "taken", TenantID: uuid.New()}
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"taken", "free"}}, nil, nil, 3, testLogger())

	link, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "free", link.Code)
}

func TestLinkService_Create_Validation(t *testing.T) {
	svc := NewLinkService(newFakeLinkRepo(), &scriptedGen{codes: []string{"c"}}, nil, nil, 5, testLogger())
	past := time.Now().Add(-time.Hour)
	zero := int64(0)

	tests := []struct {
		name string
		in   CreateLinkInput
	}{
		{"empty url", CreateLinkInput{TargetURL: ""}},
		{"ftp scheme", CreateLinkInput{TargetURL: "ftp://example.com"}},
		{"no host", CreateLinkInput{TargetURL: "http://"}},
		{"bad redirect", CreateLinkInput{TargetURL: "https://x.com", RedirectType: 307}},
		{"past expiry", CreateLinkInput{TargetURL: "https://x.com", ExpiresAt: &past}},
		{"zero max clicks", CreateLinkInput{TargetURL: "https://x.com", MaxClicks: &zero}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), uuid.New(), tt.in)
			assert.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}

func TestLinkService_Update(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"code123"}}, nil, nil, 5, testLogger())
	tenant := uuid.New()

	link, err := svc.Create(context.Background(), tenant, CreateLinkInput{TargetURL: "https://old.com"})
	require.NoError(t, err)

	newURL := "https://new.com"
	future := time.Now().Add(time.Hour)
	updated, err := svc.Update(context.Background(), tenant, link.ID, UpdateLinkInput{
		TargetURL: &newURL,
		ExpiresAt: &future,
	})
	require.NoError(t, err)
	assert.Equal(t, newURL, updated.TargetURL)
	require.NotNil(t, updated.ExpiresAt)

	// Clearing expiry.
	cleared, err := svc.Update(context.Background(), tenant, link.ID, UpdateLinkInput{ClearExpiresAt: true})
	require.NoError(t, err)
	assert.Nil(t, cleared.ExpiresAt)
}

func TestLinkService_Update_NotFound(t *testing.T) {
	svc := NewLinkService(newFakeLinkRepo(), &scriptedGen{codes: []string{"c"}}, nil, nil, 5, testLogger())
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateLinkInput{})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestLinkService_Delete(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"delme00"}}, nil, nil, 5, testLogger())
	tenant := uuid.New()

	link, err := svc.Create(context.Background(), tenant, CreateLinkInput{TargetURL: "https://x.com"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), tenant, link.ID))
	assert.ErrorIs(t, svc.Delete(context.Background(), tenant, link.ID), domain.ErrNotFound)
}

func TestLinkService_Create_PropagatesRepoError(t *testing.T) {
	repo := newFakeLinkRepo()
	repo.createErr = errors.New("db down")
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"c"}}, nil, nil, 5, testLogger())
	_, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://x.com"})
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrValidation)
}

// eventRecorder captures LinkCreated calls.
type eventRecorder struct{ created int }

func (e *eventRecorder) LinkCreated(*domain.Link) { e.created++ }

func TestLinkService_EmitsCreatedEvent(t *testing.T) {
	rec := &eventRecorder{}
	svc := NewLinkService(newFakeLinkRepo(), &scriptedGen{codes: []string{"evt0000"}}, rec, nil, 5, testLogger())
	_, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://x.com"})
	require.NoError(t, err)
	assert.Equal(t, 1, rec.created)
}

func TestLinkService_DemoPolicy_CapsExpiry(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"demo001", "demo002", "demo003"}}, nil, nil, 5, testLogger())
	base := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }
	demoTenant := uuid.New()
	svc.SetDemoPolicy(demoTenant, 24*time.Hour)
	capAt := base.Add(24 * time.Hour)

	// No expiry requested: the cap is applied.
	link, err := svc.Create(context.Background(), demoTenant, CreateLinkInput{TargetURL: "https://x.com"})
	require.NoError(t, err)
	require.NotNil(t, link.ExpiresAt)
	assert.True(t, link.ExpiresAt.Equal(capAt))

	// Expiry beyond the cap: clamped down.
	far := base.Add(30 * 24 * time.Hour)
	link, err = svc.Create(context.Background(), demoTenant, CreateLinkInput{TargetURL: "https://x.com", ExpiresAt: &far})
	require.NoError(t, err)
	require.NotNil(t, link.ExpiresAt)
	assert.True(t, link.ExpiresAt.Equal(capAt))

	// Expiry within the cap: kept as requested.
	soon := base.Add(time.Hour)
	link, err = svc.Create(context.Background(), demoTenant, CreateLinkInput{TargetURL: "https://x.com", ExpiresAt: &soon})
	require.NoError(t, err)
	require.NotNil(t, link.ExpiresAt)
	assert.True(t, link.ExpiresAt.Equal(soon))
}

func TestLinkService_DemoPolicy_OtherTenantsUnaffected(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"othr001"}}, nil, nil, 5, testLogger())
	svc.SetDemoPolicy(uuid.New(), 24*time.Hour)

	link, err := svc.Create(context.Background(), uuid.New(), CreateLinkInput{TargetURL: "https://x.com"})
	require.NoError(t, err)
	assert.Nil(t, link.ExpiresAt, "non-demo tenants keep permanent links")
}

func TestLinkService_DemoPolicy_UpdateCannotClearExpiry(t *testing.T) {
	repo := newFakeLinkRepo()
	svc := NewLinkService(repo, &scriptedGen{codes: []string{"upd0001"}}, nil, nil, 5, testLogger())
	base := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }
	demoTenant := uuid.New()
	svc.SetDemoPolicy(demoTenant, 24*time.Hour)

	link, err := svc.Create(context.Background(), demoTenant, CreateLinkInput{TargetURL: "https://x.com"})
	require.NoError(t, err)

	// Clearing the expiry on a demo link re-applies the cap instead.
	updated, err := svc.Update(context.Background(), demoTenant, link.ID, UpdateLinkInput{ClearExpiresAt: true})
	require.NoError(t, err)
	require.NotNil(t, updated.ExpiresAt)
	assert.True(t, updated.ExpiresAt.Equal(base.Add(24*time.Hour)))
}
