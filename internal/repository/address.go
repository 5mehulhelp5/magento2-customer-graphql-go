package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AddressData holds a customer_address_entity row.
type AddressData struct {
	EntityID    int
	ParentID    int
	CreatedAt   string
	UpdatedAt   string
	IsActive    int
	City        *string
	Company     *string
	CountryID   *string
	Fax         *string
	Firstname   *string
	Lastname    *string
	Middlename  *string
	Postcode    *string
	Prefix      *string
	Region      *string
	RegionID    *int
	Street      *string
	Suffix      *string
	Telephone   *string
	VatID       *string
}

type AddressRepository struct {
	db *sql.DB
}

func NewAddressRepository(db *sql.DB) *AddressRepository {
	return &AddressRepository{db: db}
}

// GetByCustomerID loads all addresses for a customer.
func (r *AddressRepository) GetByCustomerID(ctx context.Context, customerID int) ([]*AddressData, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, parent_id, created_at, updated_at, is_active,
		       city, company, country_id, fax, firstname, lastname, middlename,
		       postcode, prefix, region, region_id, street, suffix, telephone, vat_id
		FROM customer_address_entity
		WHERE parent_id = ? AND is_active = 1
		ORDER BY entity_id`,
		customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("get addresses for customer %d: %w", customerID, err)
	}
	defer rows.Close()

	var addresses []*AddressData
	for rows.Next() {
		var a AddressData
		err := rows.Scan(
			&a.EntityID, &a.ParentID, &a.CreatedAt, &a.UpdatedAt, &a.IsActive,
			&a.City, &a.Company, &a.CountryID, &a.Fax, &a.Firstname, &a.Lastname, &a.Middlename,
			&a.Postcode, &a.Prefix, &a.Region, &a.RegionID, &a.Street, &a.Suffix, &a.Telephone, &a.VatID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}
		addresses = append(addresses, &a)
	}
	return addresses, rows.Err()
}

// GetByID loads a single address.
func (r *AddressRepository) GetByID(ctx context.Context, addressID int) (*AddressData, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT entity_id, parent_id, created_at, updated_at, is_active,
		       city, company, country_id, fax, firstname, lastname, middlename,
		       postcode, prefix, region, region_id, street, suffix, telephone, vat_id
		FROM customer_address_entity
		WHERE entity_id = ?`,
		addressID,
	)

	var a AddressData
	err := row.Scan(
		&a.EntityID, &a.ParentID, &a.CreatedAt, &a.UpdatedAt, &a.IsActive,
		&a.City, &a.Company, &a.CountryID, &a.Fax, &a.Firstname, &a.Lastname, &a.Middlename,
		&a.Postcode, &a.Prefix, &a.Region, &a.RegionID, &a.Street, &a.Suffix, &a.Telephone, &a.VatID,
	)
	if err != nil {
		return nil, fmt.Errorf("address %d not found: %w", addressID, err)
	}
	return &a, nil
}

// Create inserts a new address.
func (r *AddressRepository) Create(ctx context.Context, a *AddressData) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO customer_address_entity
			(parent_id, city, company, country_id, fax, firstname, lastname, middlename,
			 postcode, prefix, region, region_id, street, suffix, telephone, vat_id,
			 is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, NOW(), NOW())`,
		a.ParentID, a.City, a.Company, a.CountryID, a.Fax,
		a.Firstname, a.Lastname, a.Middlename, a.Postcode, a.Prefix,
		a.Region, a.RegionID, a.Street, a.Suffix, a.Telephone, a.VatID,
	)
	if err != nil {
		return 0, fmt.Errorf("create address failed: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get insert id failed: %w", err)
	}
	return int(id), nil
}

// Update modifies an existing address.
func (r *AddressRepository) Update(ctx context.Context, id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	sets := make([]string, 0, len(fields)+1)
	args := make([]interface{}, 0, len(fields)+1)
	for col, val := range fields {
		sets = append(sets, col+" = ?")
		args = append(args, val)
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE customer_address_entity SET %s WHERE entity_id = ?", strings.Join(sets, ", "))
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update address %d failed: %w", id, err)
	}
	return nil
}

// Delete removes an address.
func (r *AddressRepository) Delete(ctx context.Context, addressID int) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM customer_address_entity WHERE entity_id = ?",
		addressID,
	)
	if err != nil {
		return fmt.Errorf("delete address %d failed: %w", addressID, err)
	}
	return nil
}

// GetRegion loads region details from directory_country_region.
func (r *AddressRepository) GetRegion(ctx context.Context, regionID int) (string, string, error) {
	var code, name string
	err := r.db.QueryRowContext(ctx,
		"SELECT code, default_name FROM directory_country_region WHERE region_id = ?",
		regionID,
	).Scan(&code, &name)
	if err != nil {
		return "", "", err
	}
	return code, name, nil
}
