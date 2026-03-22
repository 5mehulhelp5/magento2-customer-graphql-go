package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type GroupData struct {
	GroupID    int
	GroupCode  string
	TaxClassID int
}

type GroupRepository struct {
	db    *sql.DB
	cache map[int]*GroupData
	mu    sync.RWMutex
}

func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{
		db:    db,
		cache: make(map[int]*GroupData),
	}
}

// GetByID returns a customer group by ID, with caching.
func (r *GroupRepository) GetByID(ctx context.Context, groupID int) (*GroupData, error) {
	r.mu.RLock()
	if g, ok := r.cache[groupID]; ok {
		r.mu.RUnlock()
		return g, nil
	}
	r.mu.RUnlock()

	var g GroupData
	err := r.db.QueryRowContext(ctx,
		"SELECT customer_group_id, customer_group_code, tax_class_id FROM customer_group WHERE customer_group_id = ?",
		groupID,
	).Scan(&g.GroupID, &g.GroupCode, &g.TaxClassID)
	if err != nil {
		return nil, fmt.Errorf("customer group %d not found: %w", groupID, err)
	}

	r.mu.Lock()
	r.cache[groupID] = &g
	r.mu.Unlock()

	return &g, nil
}
