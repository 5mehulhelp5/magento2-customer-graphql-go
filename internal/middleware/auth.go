package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const CustomerIDKey contextKey = "customer_id"
const BearerTokenKey contextKey = "bearer_token"

// TokenResolver looks up customer tokens in Magento's oauth_token table.
type TokenResolver struct {
	db    *sql.DB
	cache map[string]tokenEntry
	mu    sync.RWMutex
}

type tokenEntry struct {
	customerID int
	expiresAt  time.Time
}

func NewTokenResolver(db *sql.DB) *TokenResolver {
	return &TokenResolver{
		db:    db,
		cache: make(map[string]tokenEntry),
	}
}

// Resolve returns the customer_id for a given Bearer token, or 0 if invalid/expired.
func (tr *TokenResolver) Resolve(token string) (int, error) {
	tr.mu.RLock()
	if entry, ok := tr.cache[token]; ok {
		tr.mu.RUnlock()
		if time.Now().Before(entry.expiresAt) {
			return entry.customerID, nil
		}
		// Expired cache entry, fall through to DB
		tr.mu.Lock()
		delete(tr.cache, token)
		tr.mu.Unlock()
	} else {
		tr.mu.RUnlock()
	}

	var customerID int
	err := tr.db.QueryRow(
		`SELECT customer_id FROM oauth_token
		 WHERE token = ? AND revoked = 0 AND customer_id IS NOT NULL`,
		token,
	).Scan(&customerID)
	if err != nil {
		return 0, err
	}

	tr.mu.Lock()
	tr.cache[token] = tokenEntry{
		customerID: customerID,
		expiresAt:  time.Now().Add(5 * time.Minute),
	}
	tr.mu.Unlock()

	return customerID, nil
}

// Invalidate removes a token from the cache.
func (tr *TokenResolver) Invalidate(token string) {
	tr.mu.Lock()
	delete(tr.cache, token)
	tr.mu.Unlock()
}

// AuthMiddleware extracts the Bearer token from the Authorization header
// and resolves it to a customer_id. The customer_id (or 0 for unauthenticated)
// is injected into the request context.
func AuthMiddleware(resolver *TokenResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var customerID int

			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				id, err := resolver.Resolve(token)
				if err != nil {
					log.Debug().Err(err).Msg("token resolution failed")
				} else {
					customerID = id
				}
			}

			ctx := context.WithValue(r.Context(), CustomerIDKey, customerID)
			if strings.HasPrefix(authHeader, "Bearer ") {
				ctx = context.WithValue(ctx, BearerTokenKey, strings.TrimPrefix(authHeader, "Bearer "))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetCustomerID returns the authenticated customer ID from context, or 0.
func GetCustomerID(ctx context.Context) int {
	if id, ok := ctx.Value(CustomerIDKey).(int); ok {
		return id
	}
	return 0
}

// GetBearerToken returns the raw Bearer token from context, or empty string.
func GetBearerToken(ctx context.Context) string {
	if t, ok := ctx.Value(BearerTokenKey).(string); ok {
		return t
	}
	return ""
}
