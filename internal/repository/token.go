package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

type TokenRepository struct {
	db *sql.DB
}

func NewTokenRepository(db *sql.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

// Create generates a new OAuth token for a customer and stores it in oauth_token.
func (r *TokenRepository) Create(ctx context.Context, customerID int) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	secret, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO oauth_token (customer_id, type, token, secret, callback_url, revoked, authorized, user_type, created_at)
		VALUES (?, 'access', ?, ?, '', 0, 1, 3, NOW())`,
		customerID, token, secret,
	)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}

	return token, nil
}

// Revoke marks a token as revoked.
func (r *TokenRepository) Revoke(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE oauth_token SET revoked = 1 WHERE token = ?",
		token,
	)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

// RevokeAllForCustomer revokes all tokens for a customer.
func (r *TokenRepository) RevokeAllForCustomer(ctx context.Context, customerID int) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE oauth_token SET revoked = 1 WHERE customer_id = ? AND revoked = 0",
		customerID,
	)
	if err != nil {
		return fmt.Errorf("revoke all tokens for customer %d: %w", customerID, err)
	}
	return nil
}

// GetCustomerIDByToken looks up the customer_id for an active token.
func (r *TokenRepository) GetCustomerIDByToken(ctx context.Context, token string) (int, error) {
	var customerID int
	err := r.db.QueryRowContext(ctx,
		"SELECT customer_id FROM oauth_token WHERE token = ? AND revoked = 0 AND customer_id IS NOT NULL",
		token,
	).Scan(&customerID)
	if err != nil {
		return 0, err
	}
	return customerID, nil
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
