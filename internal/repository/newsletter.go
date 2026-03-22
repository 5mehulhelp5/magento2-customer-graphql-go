package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type NewsletterRepository struct {
	db *sql.DB
}

func NewNewsletterRepository(db *sql.DB) *NewsletterRepository {
	return &NewsletterRepository{db: db}
}

// IsSubscribed checks if a customer is subscribed to the newsletter.
// Magento subscriber_status: 1=subscribed, 2=not active, 3=unsubscribed, 4=unconfirmed.
func (r *NewsletterRepository) IsSubscribed(ctx context.Context, customerID int) (bool, error) {
	var status int
	err := r.db.QueryRowContext(ctx,
		"SELECT subscriber_status FROM newsletter_subscriber WHERE customer_id = ? ORDER BY subscriber_id DESC LIMIT 1",
		customerID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("newsletter check: %w", err)
	}
	return status == 1, nil
}

// Subscribe subscribes a customer to the newsletter.
func (r *NewsletterRepository) Subscribe(ctx context.Context, customerID int, storeID int, email string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO newsletter_subscriber (store_id, customer_id, subscriber_email, subscriber_status, change_status_at)
		VALUES (?, ?, ?, 1, NOW())
		ON DUPLICATE KEY UPDATE subscriber_status = 1, change_status_at = NOW()`,
		storeID, customerID, email,
	)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

// Unsubscribe unsubscribes a customer from the newsletter.
func (r *NewsletterRepository) Unsubscribe(ctx context.Context, customerID int) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE newsletter_subscriber SET subscriber_status = 3, change_status_at = NOW() WHERE customer_id = ?",
		customerID,
	)
	if err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}
	return nil
}
