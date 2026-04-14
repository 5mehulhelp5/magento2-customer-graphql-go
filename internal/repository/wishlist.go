package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

// WishlistItemData holds one wishlist_item row joined with product info.
type WishlistItemData struct {
	ItemID    int
	Qty       float64
	AddedAt   string
	SKU       string
	Name      string
	URLKey    string
	Thumbnail string  // raw catalog path, e.g. /a/u/image.jpg
	Price     float64 // final_price from catalog_product_index_price
}

// WishlistRepository handles wishlist and wishlist_item CRUD.
type WishlistRepository struct {
	db *sql.DB
}

func NewWishlistRepository(db *sql.DB) *WishlistRepository {
	return &WishlistRepository{db: db}
}

// wishlistSharingCode generates a random 32-char hex sharing code.
func wishlistSharingCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetOrCreate returns the wishlist_id for a customer, creating one if it doesn't exist.
func (r *WishlistRepository) GetOrCreate(ctx context.Context, customerID int) (int, error) {
	var wishlistID int
	err := r.db.QueryRowContext(ctx,
		`SELECT wishlist_id FROM wishlist WHERE customer_id = ?`, customerID,
	).Scan(&wishlistID)
	if err == nil {
		return wishlistID, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("get wishlist: %w", err)
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO wishlist (customer_id, shared, sharing_code, updated_at) VALUES (?, 0, ?, NOW())`,
		customerID, wishlistSharingCode(),
	)
	if err != nil {
		return 0, fmt.Errorf("create wishlist: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("wishlist last insert id: %w", err)
	}
	return int(id), nil
}

// CountItems returns the number of items in a wishlist.
func (r *WishlistRepository) CountItems(ctx context.Context, wishlistID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wishlist_item WHERE wishlist_id = ?`, wishlistID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count wishlist items: %w", err)
	}
	return count, nil
}

// GetItems loads paginated wishlist items joined with product details.
func (r *WishlistRepository) GetItems(ctx context.Context, wishlistID, pageSize, currentPage int) ([]*WishlistItemData, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if currentPage <= 0 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			wi.wishlist_item_id,
			COALESCE(wi.qty, 1)                                                    AS qty,
			COALESCE(DATE_FORMAT(wi.added_at, '%Y-%m-%d %H:%i:%s'), '')            AS added_at,
			cpe.sku,
			COALESCE(name_v.value,   '')                                           AS name,
			COALESCE(urlkey_v.value, '')                                           AS url_key,
			COALESCE(thumb_v.value,  '')                                           AS thumbnail,
			COALESCE(pip.final_price, 0)                                           AS price
		FROM wishlist_item wi
		JOIN catalog_product_entity cpe ON cpe.entity_id = wi.product_id
		LEFT JOIN catalog_product_entity_varchar name_v
			ON  name_v.entity_id    = cpe.entity_id
			AND name_v.store_id     = 0
			AND name_v.attribute_id = (
				SELECT attribute_id FROM eav_attribute
				WHERE attribute_code = 'name' AND entity_type_id = 4
			)
		LEFT JOIN catalog_product_entity_varchar urlkey_v
			ON  urlkey_v.entity_id    = cpe.entity_id
			AND urlkey_v.store_id     = 0
			AND urlkey_v.attribute_id = (
				SELECT attribute_id FROM eav_attribute
				WHERE attribute_code = 'url_key' AND entity_type_id = 4
			)
		LEFT JOIN catalog_product_entity_varchar thumb_v
			ON  thumb_v.entity_id    = cpe.entity_id
			AND thumb_v.store_id     = 0
			AND thumb_v.attribute_id = (
				SELECT attribute_id FROM eav_attribute
				WHERE attribute_code = 'thumbnail' AND entity_type_id = 4
			)
		LEFT JOIN catalog_product_index_price pip
			ON  pip.entity_id         = cpe.entity_id
			AND pip.customer_group_id = 0
			AND pip.website_id        = 1
		WHERE wi.wishlist_id = ?
		ORDER BY wi.added_at DESC
		LIMIT ? OFFSET ?`,
		wishlistID, pageSize, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get wishlist items: %w", err)
	}
	defer rows.Close()

	var items []*WishlistItemData
	for rows.Next() {
		item := &WishlistItemData{}
		if err := rows.Scan(
			&item.ItemID,
			&item.Qty,
			&item.AddedAt,
			&item.SKU,
			&item.Name,
			&item.URLKey,
			&item.Thumbnail,
			&item.Price,
		); err != nil {
			return nil, fmt.Errorf("scan wishlist item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// AddItem adds a product (by SKU) to the wishlist.
// If the product is already in the wishlist the quantity is updated.
// Returns "product not found" when the SKU doesn't exist.
func (r *WishlistRepository) AddItem(ctx context.Context, wishlistID, storeID int, sku string, qty float64) error {
	var productID int
	if err := r.db.QueryRowContext(ctx,
		`SELECT entity_id FROM catalog_product_entity WHERE sku = ?`, sku,
	).Scan(&productID); err == sql.ErrNoRows {
		return fmt.Errorf("product not found")
	} else if err != nil {
		return fmt.Errorf("lookup product: %w", err)
	}

	// Upsert: update qty if already present, otherwise insert.
	var existingID int
	err := r.db.QueryRowContext(ctx,
		`SELECT wishlist_item_id FROM wishlist_item WHERE wishlist_id = ? AND product_id = ?`,
		wishlistID, productID,
	).Scan(&existingID)
	if err == nil {
		_, err = r.db.ExecContext(ctx,
			`UPDATE wishlist_item SET qty = ?, added_at = NOW() WHERE wishlist_item_id = ?`,
			qty, existingID,
		)
	} else if err == sql.ErrNoRows {
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO wishlist_item (wishlist_id, product_id, store_id, added_at, qty) VALUES (?, ?, ?, NOW(), ?)`,
			wishlistID, productID, storeID, qty,
		)
	}
	if err != nil {
		return fmt.Errorf("save wishlist item: %w", err)
	}
	_, _ = r.db.ExecContext(ctx,
		`UPDATE wishlist SET updated_at = NOW() WHERE wishlist_id = ?`, wishlistID,
	)
	return nil
}

// RemoveItems deletes wishlist_item rows by their IDs.
// Only deletes rows that belong to wishlistID (prevents cross-customer deletion).
func (r *WishlistRepository) RemoveItems(ctx context.Context, wishlistID int, itemIDs []int) error {
	if len(itemIDs) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(itemIDs))
	placeholders = placeholders[:len(placeholders)-1] // strip trailing comma

	args := make([]interface{}, 0, len(itemIDs)+1)
	args = append(args, wishlistID)
	for _, id := range itemIDs {
		args = append(args, id)
	}

	if _, err := r.db.ExecContext(ctx,
		fmt.Sprintf(
			`DELETE FROM wishlist_item WHERE wishlist_id = ? AND wishlist_item_id IN (%s)`,
			placeholders,
		),
		args...,
	); err != nil {
		return fmt.Errorf("remove wishlist items: %w", err)
	}
	_, _ = r.db.ExecContext(ctx,
		`UPDATE wishlist SET updated_at = NOW() WHERE wishlist_id = ?`, wishlistID,
	)
	return nil
}
