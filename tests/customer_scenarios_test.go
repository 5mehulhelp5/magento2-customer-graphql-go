package tests

// customer_scenarios_test.go — Customer creation scenarios for admin smoke-testing.
//
// Each TestCustomerScenarios sub-test creates a real customer (and addresses) via
// the Go customer GraphQL service, verifies the data returned, then writes all
// resulting customer details to magento2-admin-tests/fixtures/customers.json so
// the Playwright admin suite can verify them in the admin panel.
//
// Run:
//
//	GOTOOLCHAIN=auto go test ./tests/ -run "^TestCustomerScenarios$" -v -timeout 60s -count=1
//
// Test customers are cleaned up at the START of each run (before creation),
// so re-running is always idempotent.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ─── Fixture output ──────────────────────────────────────────────────────────

type customerFixtureEntry struct {
	Scenario     string `json:"scenario"`
	ID           string `json:"id"`
	Email        string `json:"email"`
	Firstname    string `json:"firstname"`
	Lastname     string `json:"lastname"`
	AddressCount int    `json:"addressCount,omitempty"`
}

type customerFixture struct {
	Comment   string                 `json:"_comment"`
	Customers []customerFixtureEntry `json:"customers"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mustCreateCustomer calls createCustomerV2 and returns (token, customerID).
func mustCreateCustomer(t *testing.T, email, password, firstname, lastname string) (token, id string) {
	t.Helper()
	q := fmt.Sprintf(`mutation {
		createCustomerV2(input: {
			firstname: %q
			lastname:  %q
			email:     %q
			password:  %q
		}) {
			customer { id email firstname lastname }
		}
	}`, firstname, lastname, email, password)
	resp := doQuery(t, q, "")
	if len(resp.Errors) > 0 {
		t.Fatalf("createCustomerV2 %s: %s", email, resp.Errors[0].Message)
	}
	var d struct {
		CreateCustomerV2 struct {
			Customer struct {
				ID        string `json:"id"`
				Email     string `json:"email"`
				Firstname string `json:"firstname"`
				Lastname  string `json:"lastname"`
			} `json:"customer"`
		} `json:"createCustomerV2"`
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatalf("unmarshal createCustomerV2: %v", err)
	}
	if d.CreateCustomerV2.Customer.Email != email {
		t.Fatalf("expected email %s, got %s", email, d.CreateCustomerV2.Customer.Email)
	}
	t.Logf("  created customer: %s %s <%s> id=%s",
		d.CreateCustomerV2.Customer.Firstname,
		d.CreateCustomerV2.Customer.Lastname,
		d.CreateCustomerV2.Customer.Email,
		d.CreateCustomerV2.Customer.ID,
	)

	// Generate token so callers can make authenticated calls
	tok := mustGenerateToken(t, email, password)
	return tok, d.CreateCustomerV2.Customer.ID
}

// mustGenerateToken generates a customer token for the given credentials.
func mustGenerateToken(t *testing.T, email, password string) string {
	t.Helper()
	q := fmt.Sprintf(`mutation {
		generateCustomerToken(email: %q, password: %q) { token }
	}`, email, password)
	resp := doQuery(t, q, "")
	if len(resp.Errors) > 0 {
		t.Fatalf("generateCustomerToken %s: %s", email, resp.Errors[0].Message)
	}
	var d struct {
		GenerateCustomerToken struct {
			Token string `json:"token"`
		} `json:"generateCustomerToken"`
	}
	json.Unmarshal(resp.Data, &d)
	if d.GenerateCustomerToken.Token == "" {
		t.Fatalf("generateCustomerToken returned empty token for %s", email)
	}
	return d.GenerateCustomerToken.Token
}

// mustCreateAddress creates an address for the authenticated customer.
// Returns the address ID.
func mustCreateAddress(t *testing.T, token string, addr customerAddressInput) int {
	t.Helper()
	defaultStr := ""
	if addr.DefaultShipping {
		defaultStr += " default_shipping: true"
	}
	if addr.DefaultBilling {
		defaultStr += " default_billing: true"
	}
	q := fmt.Sprintf(`mutation {
		createCustomerAddress(input: {
			firstname:    %q
			lastname:     %q
			street:       [%q]
			city:         %q
			region:       { region_code: %q, region_id: %d }
			postcode:     %q
			country_code: US
			telephone:    %q
			%s
		}) {
			id uid firstname lastname city street
			region { region_code region_id }
			default_shipping default_billing
		}
	}`, addr.Firstname, addr.Lastname, addr.Street, addr.City,
		addr.RegionCode, addr.RegionID, addr.Postcode, addr.Telephone, defaultStr)
	resp := doQuery(t, q, token)
	if len(resp.Errors) > 0 {
		t.Fatalf("createCustomerAddress: %s", resp.Errors[0].Message)
	}
	var d struct {
		CreateCustomerAddress struct {
			ID   int    `json:"id"`
			UID  string `json:"uid"`
			City string `json:"city"`
		} `json:"createCustomerAddress"`
	}
	json.Unmarshal(resp.Data, &d)
	if d.CreateCustomerAddress.ID == 0 {
		t.Fatal("createCustomerAddress returned id=0")
	}
	t.Logf("  created address id=%d city=%s", d.CreateCustomerAddress.ID, d.CreateCustomerAddress.City)
	return d.CreateCustomerAddress.ID
}

type customerAddressInput struct {
	Firstname       string
	Lastname        string
	Street          string
	City            string
	RegionCode      string
	RegionID        int
	Postcode        string
	Telephone       string
	DefaultShipping bool
	DefaultBilling  bool
}

// mustGetCustomerProfile queries the customer profile and returns id + address count.
func mustGetCustomerProfile(t *testing.T, token string) (id string, addressCount int) {
	t.Helper()
	resp := doQuery(t, `{
		customer {
			id email firstname lastname
			addresses { id city default_shipping default_billing }
		}
	}`, token)
	if len(resp.Errors) > 0 {
		t.Fatalf("customer query: %s", resp.Errors[0].Message)
	}
	var d struct {
		Customer struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Firstname string `json:"firstname"`
			Addresses []struct {
				ID              int    `json:"id"`
				City            string `json:"city"`
				DefaultShipping bool   `json:"default_shipping"`
				DefaultBilling  bool   `json:"default_billing"`
			} `json:"addresses"`
		} `json:"customer"`
	}
	json.Unmarshal(resp.Data, &d)
	t.Logf("  profile: id=%s email=%s addresses=%d",
		d.Customer.ID, d.Customer.Email, len(d.Customer.Addresses))
	return d.Customer.ID, len(d.Customer.Addresses)
}

// cleanupScenarioCustomers removes previously created scenario customers from the DB.
func cleanupScenarioCustomers(t *testing.T, emailPattern string) {
	t.Helper()
	if testDB == nil {
		return
	}
	// Delete flat grid entries
	testDB.Exec(`DELETE FROM customer_grid_flat WHERE email LIKE ?`, emailPattern)
	// Delete addresses first (no FK cascade in Magento by default)
	testDB.Exec(`
		DELETE cae FROM customer_address_entity cae
		INNER JOIN customer_entity ce ON cae.parent_id = ce.entity_id
		WHERE ce.email LIKE ?`, emailPattern)
	res, _ := testDB.Exec(`DELETE FROM customer_entity WHERE email LIKE ?`, emailPattern)
	if n, _ := res.RowsAffected(); n > 0 {
		t.Logf("  cleaned up %d previous scenario customer(s)", n)
	}
}

// syncCustomerGridFlat inserts rows into customer_grid_flat for scenario customers
// so they appear in the Magento admin customer grid (which reads from this flat table,
// not customer_entity directly).
//
// The flat table is normally populated by the Magento customer_grid indexer. Our Go
// service writes only to customer_entity, so we must backfill it for admin tests.
func syncCustomerGridFlat(t *testing.T, emailPattern string) {
	t.Helper()
	if testDB == nil {
		return
	}
	_, err := testDB.Exec(`
		INSERT INTO customer_grid_flat
		  (entity_id, name, email, group_id, created_at, website_id,
		   confirmation, created_in, dob, gender, taxvat)
		SELECT
		  ce.entity_id,
		  CONCAT(ce.firstname, ' ', ce.lastname),
		  ce.email,
		  COALESCE(ce.group_id, 1),
		  ce.created_at,
		  COALESCE(ce.website_id, 1),
		  ce.confirmation,
		  'Default Store View',
		  ce.dob,
		  ce.gender,
		  ce.taxvat
		FROM customer_entity ce
		WHERE ce.email LIKE ?
		ON DUPLICATE KEY UPDATE
		  name        = VALUES(name),
		  email       = VALUES(email),
		  group_id    = VALUES(group_id),
		  created_at  = VALUES(created_at),
		  website_id  = VALUES(website_id)
	`, emailPattern)
	if err != nil {
		t.Logf("  WARNING: could not sync customer_grid_flat: %v", err)
		return
	}
	t.Logf("  synced customer_grid_flat for pattern: %s", emailPattern)
}

// writeCustomerFixture writes the customer fixture JSON to the admin-tests directory.
func writeCustomerFixture(t *testing.T, fixture customerFixture) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// tests/ -> magento2-customer-graphql-go/ -> magento2-graphql-go/ -> magento2-admin-tests/fixtures/
	root := filepath.Dir(filepath.Dir(thisFile))
	fixturePath := filepath.Join(root, "..", "magento2-admin-tests", "fixtures", "customers.json")
	fixturePath = filepath.Clean(fixturePath)

	if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(fixturePath, data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", fixturePath, err)
	}
	t.Logf("customer fixture written to: %s", fixturePath)
}

// ─── Scenario runners ─────────────────────────────────────────────────────────

// runC01 creates a basic customer with just name + email + password.
func runC01(t *testing.T, suffix string) customerFixtureEntry {
	t.Helper()
	email := fmt.Sprintf("scenario-customer-%s-basic@example.com", suffix)
	token, id := mustCreateCustomer(t, email, "Scenario1!", "Basic", "Scenario")

	profileID, addrCount := mustGetCustomerProfile(t, token)
	if profileID != id {
		t.Errorf("C01: profile id=%s, create returned id=%s", profileID, id)
	}
	if addrCount != 0 {
		t.Errorf("C01: expected 0 addresses, got %d", addrCount)
	}

	return customerFixtureEntry{
		Scenario:  "C01_basic",
		ID:        id,
		Email:     email,
		Firstname: "Basic",
		Lastname:  "Scenario",
	}
}

// runC02 creates a customer with optional profile fields: date_of_birth, gender,
// and taxvat (tax/VAT ID).
func runC02(t *testing.T, suffix string) customerFixtureEntry {
	t.Helper()
	email := fmt.Sprintf("scenario-customer-%s-profile@example.com", suffix)

	// Create customer
	q := fmt.Sprintf(`mutation {
		createCustomerV2(input: {
			firstname:     "Profile"
			lastname:      "Scenario"
			email:         %q
			password:      "Scenario2!"
			date_of_birth: "1990-06-15"
			gender:        1
			taxvat:        "US123456789"
		}) {
			customer { id email firstname lastname date_of_birth gender taxvat }
		}
	}`, email)
	resp := doQuery(t, q, "")
	if len(resp.Errors) > 0 {
		t.Fatalf("createCustomerV2 C02: %s", resp.Errors[0].Message)
	}
	var d struct {
		CreateCustomerV2 struct {
			Customer struct {
				ID          string `json:"id"`
				Email       string `json:"email"`
				DateOfBirth string `json:"date_of_birth"`
				Gender      *int   `json:"gender"`
				Taxvat      string `json:"taxvat"`
			} `json:"customer"`
		} `json:"createCustomerV2"`
	}
	json.Unmarshal(resp.Data, &d)
	if d.CreateCustomerV2.Customer.DateOfBirth == "" {
		t.Error("C02: expected non-empty date_of_birth")
	}
	if d.CreateCustomerV2.Customer.Gender == nil || *d.CreateCustomerV2.Customer.Gender != 1 {
		t.Error("C02: expected gender=1")
	}
	if d.CreateCustomerV2.Customer.Taxvat != "US123456789" {
		t.Errorf("C02: expected taxvat=US123456789, got %s", d.CreateCustomerV2.Customer.Taxvat)
	}
	t.Logf("  C02 customer dob=%s gender=%v taxvat=%s",
		d.CreateCustomerV2.Customer.DateOfBirth,
		d.CreateCustomerV2.Customer.Gender,
		d.CreateCustomerV2.Customer.Taxvat,
	)

	return customerFixtureEntry{
		Scenario:  "C02_profile_fields",
		ID:        d.CreateCustomerV2.Customer.ID,
		Email:     email,
		Firstname: "Profile",
		Lastname:  "Scenario",
	}
}

// runC03 creates a customer with two addresses: a default shipping (home) and a
// default billing (work).
func runC03(t *testing.T, suffix string) customerFixtureEntry {
	t.Helper()
	email := fmt.Sprintf("scenario-customer-%s-addresses@example.com", suffix)
	token, id := mustCreateCustomer(t, email, "Scenario3!", "Address", "Scenario")

	// Home address — default shipping
	mustCreateAddress(t, token, customerAddressInput{
		Firstname:       "Address",
		Lastname:        "Scenario",
		Street:          "742 Evergreen Terrace",
		City:            "Springfield",
		RegionCode:      "IL",
		RegionID:        14, // Illinois
		Postcode:        "62704",
		Telephone:       "2175550100",
		DefaultShipping: true,
		DefaultBilling:  false,
	})

	// Work address — default billing
	mustCreateAddress(t, token, customerAddressInput{
		Firstname:       "Address",
		Lastname:        "Scenario",
		Street:          "100 Business Pkwy",
		City:            "Chicago",
		RegionCode:      "IL",
		RegionID:        14,
		Postcode:        "60601",
		Telephone:       "3125550200",
		DefaultShipping: false,
		DefaultBilling:  true,
	})

	_, addrCount := mustGetCustomerProfile(t, token)
	if addrCount != 2 {
		t.Errorf("C03: expected 2 addresses, got %d", addrCount)
	}

	return customerFixtureEntry{
		Scenario:     "C03_two_addresses",
		ID:           id,
		Email:        email,
		Firstname:    "Address",
		Lastname:     "Scenario",
		AddressCount: 2,
	}
}

// ─── TestCustomerScenarios ────────────────────────────────────────────────────

// TestCustomerScenarios creates three customer accounts via the Go customer
// GraphQL service, verifies the returned data, then writes a fixture file for
// the Playwright admin tests to consume.
func TestCustomerScenarios(t *testing.T) {
	// Use a short timestamp suffix so fixture emails are stable within a run
	// but previous runs are cleaned up cleanly.
	suffix := time.Now().Format("20060102")

	t.Log("=== Cleaning up previous scenario customers ===")
	cleanupScenarioCustomers(t, "scenario-customer-%@example.com")

	entries := make([]customerFixtureEntry, 3)

	t.Run("C01_basic_customer", func(t *testing.T) {
		entries[0] = runC01(t, suffix)
	})

	t.Run("C02_profile_fields", func(t *testing.T) {
		entries[1] = runC02(t, suffix)
	})

	t.Run("C03_two_addresses", func(t *testing.T) {
		entries[2] = runC03(t, suffix)
	})

	// Sync customer_grid_flat so scenario customers appear in the Magento admin grid.
	// The Go service writes to customer_entity only; the flat table is normally updated
	// by Magento's customer_grid indexer.
	syncCustomerGridFlat(t, "scenario-customer-%@example.com")

	// Write fixture regardless of sub-test failures
	fixture := customerFixture{
		Comment:   fmt.Sprintf("Written by TestCustomerScenarios on %s", time.Now().Format(time.RFC3339)),
		Customers: entries,
	}
	writeCustomerFixture(t, fixture)

	t.Log("=== Customer fixture written ===")
	for _, e := range entries {
		t.Logf("  %s: %s <%s> (addresses: %d)", e.Scenario, e.ID, e.Email, e.AddressCount)
	}
}
