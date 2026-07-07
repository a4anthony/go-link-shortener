package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// TenantRepository persists tenants.
type TenantRepository struct {
	db Querier
}

// NewTenantRepository builds a TenantRepository over the given pool.
func NewTenantRepository(db Querier) *TenantRepository {
	return &TenantRepository{db: db}
}

// Create inserts a new tenant and returns it with generated fields populated.
func (r *TenantRepository) Create(ctx context.Context, name string) (*domain.Tenant, error) {
	const q = `
		INSERT INTO tenants (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at`
	var t domain.Tenant
	err := r.db.QueryRow(ctx, q, name).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert tenant: %w", err)
	}
	return &t, nil
}

// GetByID loads a tenant by id, returning domain.ErrNotFound if absent.
func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	const q = `
		SELECT id, name, created_at, updated_at
		FROM tenants
		WHERE id = $1`
	var t domain.Tenant
	err := r.db.QueryRow(ctx, q, id).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select tenant: %w", err)
	}
	return &t, nil
}

// FindByName returns the first tenant with the given name, or ErrNotFound. Used
// by the dev seed to stay idempotent across restarts.
func (r *TenantRepository) FindByName(ctx context.Context, name string) (*domain.Tenant, error) {
	const q = `
		SELECT id, name, created_at, updated_at
		FROM tenants
		WHERE name = $1
		ORDER BY created_at
		LIMIT 1`
	var t domain.Tenant
	err := r.db.QueryRow(ctx, q, name).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select tenant by name: %w", err)
	}
	return &t, nil
}
