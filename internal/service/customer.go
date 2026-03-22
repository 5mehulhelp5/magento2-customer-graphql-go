package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/magendooro/magento2-customer-graphql-go/graph/model"
	"github.com/magendooro/magento2-customer-graphql-go/internal/middleware"
	"github.com/magendooro/magento2-customer-graphql-go/internal/repository"
)

type CustomerService struct {
	customerRepo   *repository.CustomerRepository
	addressRepo    *repository.AddressRepository
	tokenRepo      *repository.TokenRepository
	newsletterRepo *repository.NewsletterRepository
	storeRepo      *repository.StoreRepository
}

func NewCustomerService(
	customerRepo *repository.CustomerRepository,
	addressRepo *repository.AddressRepository,
	tokenRepo *repository.TokenRepository,
	newsletterRepo *repository.NewsletterRepository,
	storeRepo *repository.StoreRepository,
) *CustomerService {
	return &CustomerService{
		customerRepo:   customerRepo,
		addressRepo:    addressRepo,
		tokenRepo:      tokenRepo,
		newsletterRepo: newsletterRepo,
		storeRepo:      storeRepo,
	}
}

// GetCustomer returns the authenticated customer's data.
func (s *CustomerService) GetCustomer(ctx context.Context) (*model.Customer, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	data, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	customer := s.mapCustomer(data)

	// Load addresses
	addresses, err := s.addressRepo.GetByCustomerID(ctx, customerID)
	if err != nil {
		log.Warn().Err(err).Int("customer_id", customerID).Msg("failed to load addresses")
	} else {
		customer.Addresses = s.mapAddresses(addresses, data.DefaultBilling, data.DefaultShipping)
	}

	// Check newsletter subscription
	subscribed, err := s.newsletterRepo.IsSubscribed(ctx, customerID)
	if err != nil {
		log.Warn().Err(err).Int("customer_id", customerID).Msg("failed to check newsletter")
	}
	customer.IsSubscribed = &subscribed

	return customer, nil
}

// GenerateToken authenticates a customer and returns a token.
func (s *CustomerService) GenerateToken(ctx context.Context, email, password string) (*model.CustomerToken, error) {
	storeID := middleware.GetStoreID(ctx)
	websiteID, _ := s.storeRepo.GetWebsiteIDForStore(ctx, storeID)

	data, err := s.customerRepo.GetByEmail(ctx, email, websiteID)
	if err != nil {
		return nil, fmt.Errorf("the account sign-in was incorrect or your account is disabled temporarily. Please wait and try again later")
	}

	if data.IsActive != 1 {
		return nil, fmt.Errorf("the account sign-in was incorrect or your account is disabled temporarily. Please wait and try again later")
	}

	if !repository.VerifyPassword(data.PasswordHash, password) {
		return nil, fmt.Errorf("the account sign-in was incorrect or your account is disabled temporarily. Please wait and try again later")
	}

	token, err := s.tokenRepo.Create(ctx, data.EntityID)
	if err != nil {
		return nil, fmt.Errorf("token generation failed: %w", err)
	}

	return &model.CustomerToken{Token: &token}, nil
}

// RevokeToken revokes the current customer's token.
func (s *CustomerService) RevokeToken(ctx context.Context) (*model.RevokeCustomerTokenOutput, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	err := s.tokenRepo.RevokeAllForCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("token revocation failed: %w", err)
	}

	result := true
	return &model.RevokeCustomerTokenOutput{Result: result}, nil
}

// IsEmailAvailable checks if an email can be used for registration.
func (s *CustomerService) IsEmailAvailable(ctx context.Context, email string) (*model.IsEmailAvailableOutput, error) {
	storeID := middleware.GetStoreID(ctx)
	websiteID, _ := s.storeRepo.GetWebsiteIDForStore(ctx, storeID)

	exists, err := s.customerRepo.EmailExists(ctx, email, websiteID)
	if err != nil {
		return nil, err
	}

	available := !exists
	return &model.IsEmailAvailableOutput{IsEmailAvailable: &available}, nil
}

// CreateCustomer registers a new customer account.
func (s *CustomerService) CreateCustomer(ctx context.Context, input model.CustomerCreateInput) (*model.CustomerOutput, error) {
	storeID := middleware.GetStoreID(ctx)
	websiteID, _ := s.storeRepo.GetWebsiteIDForStore(ctx, storeID)

	exists, err := s.customerRepo.EmailExists(ctx, input.Email, websiteID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("a customer with the same email address already exists in an associated website")
	}

	passwordHash, err := repository.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	data := &repository.CustomerData{
		WebsiteID:    websiteID,
		Email:        input.Email,
		GroupID:      1, // General
		StoreID:      storeID,
		Firstname:    &input.Firstname,
		Lastname:     &input.Lastname,
		Prefix:       input.Prefix,
		Middlename:   input.Middlename,
		Suffix:       input.Suffix,
		Dob:          input.DateOfBirth,
		Taxvat:       input.Taxvat,
		Gender:       input.Gender,
		PasswordHash: passwordHash,
	}

	id, err := s.customerRepo.Create(ctx, data)
	if err != nil {
		return nil, err
	}

	// Handle newsletter subscription
	if input.IsSubscribed != nil && *input.IsSubscribed {
		if err := s.newsletterRepo.Subscribe(ctx, id, storeID, input.Email); err != nil {
			log.Warn().Err(err).Int("customer_id", id).Msg("newsletter subscribe failed")
		}
	}

	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := s.mapCustomer(customer)
	return &model.CustomerOutput{Customer: result}, nil
}

// UpdateCustomer updates the authenticated customer's profile.
func (s *CustomerService) UpdateCustomer(ctx context.Context, input model.CustomerUpdateInput) (*model.CustomerOutput, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	fields := make(map[string]interface{})
	if input.Firstname != nil {
		fields["firstname"] = *input.Firstname
	}
	if input.Lastname != nil {
		fields["lastname"] = *input.Lastname
	}
	if input.Middlename != nil {
		fields["middlename"] = *input.Middlename
	}
	if input.Prefix != nil {
		fields["prefix"] = *input.Prefix
	}
	if input.Suffix != nil {
		fields["suffix"] = *input.Suffix
	}
	if input.DateOfBirth != nil {
		fields["dob"] = *input.DateOfBirth
	}
	if input.Taxvat != nil {
		fields["taxvat"] = *input.Taxvat
	}
	if input.Gender != nil {
		fields["gender"] = *input.Gender
	}

	if err := s.customerRepo.Update(ctx, customerID, fields); err != nil {
		return nil, err
	}

	// Handle newsletter
	if input.IsSubscribed != nil {
		storeID := middleware.GetStoreID(ctx)
		customer, _ := s.customerRepo.GetByID(ctx, customerID)
		if *input.IsSubscribed {
			s.newsletterRepo.Subscribe(ctx, customerID, storeID, customer.Email)
		} else {
			s.newsletterRepo.Unsubscribe(ctx, customerID)
		}
	}

	customer, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	result := s.mapCustomer(customer)
	return &model.CustomerOutput{Customer: result}, nil
}

// ChangePassword changes the authenticated customer's password.
func (s *CustomerService) ChangePassword(ctx context.Context, currentPassword, newPassword string) (*model.Customer, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	data, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	if !repository.VerifyPassword(data.PasswordHash, currentPassword) {
		return nil, fmt.Errorf("the password doesn't match this account. Verify the password and try again")
	}

	hash, err := repository.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	if err := s.customerRepo.Update(ctx, customerID, map[string]interface{}{
		"password_hash": hash,
	}); err != nil {
		return nil, err
	}

	// Revoke existing tokens for security
	s.tokenRepo.RevokeAllForCustomer(ctx, customerID)

	updated, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return s.mapCustomer(updated), nil
}

// UpdateEmail changes the authenticated customer's email (requires password verification).
func (s *CustomerService) UpdateEmail(ctx context.Context, email, password string) (*model.CustomerOutput, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	data, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	if !repository.VerifyPassword(data.PasswordHash, password) {
		return nil, fmt.Errorf("the password doesn't match this account. Verify the password and try again")
	}

	if err := s.customerRepo.Update(ctx, customerID, map[string]interface{}{
		"email": email,
	}); err != nil {
		return nil, err
	}

	updated, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	result := s.mapCustomer(updated)
	return &model.CustomerOutput{Customer: result}, nil
}

// CreateAddress creates a new address for the authenticated customer.
func (s *CustomerService) CreateAddress(ctx context.Context, input model.CustomerAddressInput) (*model.CustomerAddress, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	data := s.mapAddressInput(input)
	data.ParentID = customerID

	id, err := s.addressRepo.Create(ctx, data)
	if err != nil {
		return nil, err
	}

	// Set as default if requested
	if input.DefaultBilling != nil && *input.DefaultBilling {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_billing": id})
	}
	if input.DefaultShipping != nil && *input.DefaultShipping {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_shipping": id})
	}

	created, err := s.addressRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	customer, _ := s.customerRepo.GetByID(ctx, customerID)
	return s.mapAddress(created, customer.DefaultBilling, customer.DefaultShipping), nil
}

// UpdateAddress updates an existing address.
func (s *CustomerService) UpdateAddress(ctx context.Context, addressID int, input model.CustomerAddressInput) (*model.CustomerAddress, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return nil, fmt.Errorf("the current customer isn't authorized")
	}

	// Verify ownership
	existing, err := s.addressRepo.GetByID(ctx, addressID)
	if err != nil {
		return nil, fmt.Errorf("address not found: %w", err)
	}
	if existing.ParentID != customerID {
		return nil, fmt.Errorf("address doesn't belong to this customer")
	}

	fields := s.mapAddressFields(input)
	if err := s.addressRepo.Update(ctx, addressID, fields); err != nil {
		return nil, err
	}

	// Handle default flags
	if input.DefaultBilling != nil && *input.DefaultBilling {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_billing": addressID})
	}
	if input.DefaultShipping != nil && *input.DefaultShipping {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_shipping": addressID})
	}

	updated, err := s.addressRepo.GetByID(ctx, addressID)
	if err != nil {
		return nil, err
	}

	customer, _ := s.customerRepo.GetByID(ctx, customerID)
	return s.mapAddress(updated, customer.DefaultBilling, customer.DefaultShipping), nil
}

// DeleteAddress removes an address.
func (s *CustomerService) DeleteAddress(ctx context.Context, addressID int) (bool, error) {
	customerID := middleware.GetCustomerID(ctx)
	if customerID == 0 {
		return false, fmt.Errorf("the current customer isn't authorized")
	}

	existing, err := s.addressRepo.GetByID(ctx, addressID)
	if err != nil {
		return false, fmt.Errorf("address not found: %w", err)
	}
	if existing.ParentID != customerID {
		return false, fmt.Errorf("address doesn't belong to this customer")
	}

	// Clear default references if needed
	customer, _ := s.customerRepo.GetByID(ctx, customerID)
	if customer.DefaultBilling != nil && *customer.DefaultBilling == addressID {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_billing": nil})
	}
	if customer.DefaultShipping != nil && *customer.DefaultShipping == addressID {
		s.customerRepo.Update(ctx, customerID, map[string]interface{}{"default_shipping": nil})
	}

	if err := s.addressRepo.Delete(ctx, addressID); err != nil {
		return false, err
	}
	return true, nil
}

// ── Mapping helpers ──────────────────────────────────────────────────────────

func (s *CustomerService) mapCustomer(data *repository.CustomerData) *model.Customer {
	id := strconv.Itoa(data.EntityID)
	createdAt := formatDateTime(data.CreatedAt)

	var defaultBilling, defaultShipping *string
	if data.DefaultBilling != nil {
		v := strconv.Itoa(*data.DefaultBilling)
		defaultBilling = &v
	}
	if data.DefaultShipping != nil {
		v := strconv.Itoa(*data.DefaultShipping)
		defaultShipping = &v
	}

	confirmStatus := model.ConfirmationStatusEnumAccountConfirmationNotRequired
	if data.Confirmation != nil && *data.Confirmation != "" {
		confirmStatus = model.ConfirmationStatusEnumAccountConfirmed
	}

	var dob *string
	if data.Dob != nil && *data.Dob != "" {
		d := formatDate(*data.Dob)
		dob = &d
	}

	return &model.Customer{
		ID:                 id,
		Firstname:          data.Firstname,
		Lastname:           data.Lastname,
		Middlename:         data.Middlename,
		Prefix:             data.Prefix,
		Suffix:             data.Suffix,
		Email:              &data.Email,
		DateOfBirth:        dob,
		Taxvat:             data.Taxvat,
		Gender:             data.Gender,
		CreatedAt:          &createdAt,
		DefaultBilling:     defaultBilling,
		DefaultShipping:    defaultShipping,
		ConfirmationStatus: confirmStatus,
		GroupID:            &data.GroupID,
	}
}

// formatDate converts a datetime string to YYYY-MM-DD (Magento dob format).
func formatDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// formatDateTime converts a datetime string to "YYYY-MM-DD HH:MM:SS" (Magento created_at format).
func formatDateTime(s string) string {
	// MySQL parseTime gives "2006-01-02T15:04:05Z" or "2006-01-02 15:04:05"
	if len(s) >= 19 {
		// Replace T with space and strip timezone suffix
		result := strings.Replace(s[:19], "T", " ", 1)
		return result
	}
	return s
}

func (s *CustomerService) mapAddresses(addrs []*repository.AddressData, defaultBilling, defaultShipping *int) []*model.CustomerAddress {
	result := make([]*model.CustomerAddress, len(addrs))
	for i, a := range addrs {
		result[i] = s.mapAddress(a, defaultBilling, defaultShipping)
	}
	return result
}

func (s *CustomerService) mapAddress(a *repository.AddressData, defaultBilling, defaultShipping *int) *model.CustomerAddress {
	uid := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(a.EntityID)))
	id := a.EntityID

	isDefaultBilling := defaultBilling != nil && *defaultBilling == a.EntityID
	isDefaultShipping := defaultShipping != nil && *defaultShipping == a.EntityID

	var region *model.CustomerAddressRegion
	if a.RegionID != nil || a.Region != nil {
		region = &model.CustomerAddressRegion{
			Region:   a.Region,
			RegionID: a.RegionID,
		}
		// Try to resolve region code
		if a.RegionID != nil && *a.RegionID > 0 {
			if code, _, err := s.addressRepo.GetRegion(context.Background(), *a.RegionID); err == nil {
				region.RegionCode = &code
			}
		}
	}

	var countryCode *model.CountryCodeEnum
	if a.CountryID != nil {
		cc := model.CountryCodeEnum(*a.CountryID)
		if cc.IsValid() {
			countryCode = &cc
		}
	}

	// Parse street (stored as newline-separated in Magento)
	var street []*string
	if a.Street != nil {
		lines := strings.Split(*a.Street, "\n")
		for _, l := range lines {
			line := l
			street = append(street, &line)
		}
	}

	return &model.CustomerAddress{
		ID:              &id,
		UID:             uid,
		Firstname:       a.Firstname,
		Lastname:        a.Lastname,
		Middlename:      a.Middlename,
		Prefix:          a.Prefix,
		Suffix:          a.Suffix,
		Company:         a.Company,
		Street:          street,
		City:            a.City,
		Region:          region,
		RegionID:        a.RegionID,
		Postcode:        a.Postcode,
		CountryCode:     countryCode,
		CountryID:       a.CountryID,
		Telephone:       a.Telephone,
		Fax:             a.Fax,
		VatID:           a.VatID,
		DefaultShipping: &isDefaultShipping,
		DefaultBilling:  &isDefaultBilling,
	}
}

func (s *CustomerService) mapAddressInput(input model.CustomerAddressInput) *repository.AddressData {
	a := &repository.AddressData{
		Firstname:  input.Firstname,
		Lastname:   input.Lastname,
		Middlename: input.Middlename,
		Prefix:     input.Prefix,
		Suffix:     input.Suffix,
		Company:    input.Company,
		City:       input.City,
		Postcode:   input.Postcode,
		Telephone:  input.Telephone,
		Fax:        input.Fax,
		VatID:      input.VatID,
	}

	if input.CountryCode != nil {
		cc := string(*input.CountryCode)
		a.CountryID = &cc
	}

	if input.Street != nil {
		lines := make([]string, len(input.Street))
		for i, s := range input.Street {
			if s != nil {
				lines[i] = *s
			}
		}
		street := strings.Join(lines, "\n")
		a.Street = &street
	}

	if input.Region != nil {
		a.Region = input.Region.Region
		a.RegionID = input.Region.RegionID
	}

	return a
}

func (s *CustomerService) mapAddressFields(input model.CustomerAddressInput) map[string]interface{} {
	fields := make(map[string]interface{})
	if input.Firstname != nil {
		fields["firstname"] = *input.Firstname
	}
	if input.Lastname != nil {
		fields["lastname"] = *input.Lastname
	}
	if input.Middlename != nil {
		fields["middlename"] = *input.Middlename
	}
	if input.Prefix != nil {
		fields["prefix"] = *input.Prefix
	}
	if input.Suffix != nil {
		fields["suffix"] = *input.Suffix
	}
	if input.Company != nil {
		fields["company"] = *input.Company
	}
	if input.City != nil {
		fields["city"] = *input.City
	}
	if input.Postcode != nil {
		fields["postcode"] = *input.Postcode
	}
	if input.Telephone != nil {
		fields["telephone"] = *input.Telephone
	}
	if input.Fax != nil {
		fields["fax"] = *input.Fax
	}
	if input.VatID != nil {
		fields["vat_id"] = *input.VatID
	}
	if input.CountryCode != nil {
		fields["country_id"] = string(*input.CountryCode)
	}
	if input.Street != nil {
		lines := make([]string, len(input.Street))
		for i, s := range input.Street {
			if s != nil {
				lines[i] = *s
			}
		}
		fields["street"] = strings.Join(lines, "\n")
	}
	if input.Region != nil {
		if input.Region.Region != nil {
			fields["region"] = *input.Region.Region
		}
		if input.Region.RegionID != nil {
			fields["region_id"] = *input.Region.RegionID
		}
	}
	return fields
}
