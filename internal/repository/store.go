package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type StoreData struct {
	StoreID   int
	Code      string
	WebsiteID int
	Name      string
}

type StoreRepository struct {
	db    *sql.DB
	cache map[int]*StoreData
	mu    sync.RWMutex
}

func NewStoreRepository(db *sql.DB) *StoreRepository {
	return &StoreRepository{
		db:    db,
		cache: make(map[int]*StoreData),
	}
}

// GetByID loads store data by store_id, with caching.
func (r *StoreRepository) GetByID(ctx context.Context, storeID int) (*StoreData, error) {
	r.mu.RLock()
	if s, ok := r.cache[storeID]; ok {
		r.mu.RUnlock()
		return s, nil
	}
	r.mu.RUnlock()

	var s StoreData
	err := r.db.QueryRowContext(ctx,
		"SELECT store_id, code, website_id, name FROM store WHERE store_id = ?",
		storeID,
	).Scan(&s.StoreID, &s.Code, &s.WebsiteID, &s.Name)
	if err != nil {
		return nil, fmt.Errorf("store %d not found: %w", storeID, err)
	}

	r.mu.Lock()
	r.cache[storeID] = &s
	r.mu.Unlock()

	return &s, nil
}

// GetWebsiteIDForStore returns the website_id for a given store_id.
func (r *StoreRepository) GetWebsiteIDForStore(ctx context.Context, storeID int) (int, error) {
	s, err := r.GetByID(ctx, storeID)
	if err != nil {
		return 1, err // default website_id = 1
	}
	return s.WebsiteID, nil
}
