---
name: add-attribute
description: Add a new EAV customer attribute to the GraphQL schema. Use when adding custom Magento customer attributes to the API.
argument-hint: <attribute_code>
---

Add the Magento customer EAV attribute `$ARGUMENTS` to the GraphQL API. Follow these steps exactly:

## 1. Schema

Add the field to `Customer` type in `graph/schema.graphqls`. Use the appropriate GraphQL type:
- `varchar` / `text` backend → `String`
- `int` backend → `Int`
- `decimal` backend → `Float`
- `datetime` backend → `String`

## 2. Regenerate

```bash
go run github.com/99designs/gqlgen generate
```

## 3. Repository

In `internal/repository/customer.go`:
- Add the field to the `CustomerData` struct
- Add the column or EAV JOIN in `GetCustomerByID()`
- For flat columns: just add to SELECT
- For EAV attributes: JOIN `customer_entity_<backend_type>` using `entity_id` with `COALESCE(store_value, default_value)` for store scoping

## 4. Service mapping

In `internal/service/customer.go`, in the mapping function, map the new field from `CustomerData` to the generated model type.

## 5. Verify

```bash
go build ./...
go vet ./...
```

## Important

- Customer entity uses `entity_id` (NOT `row_id` — that's catalog only)
- The attribute must exist in Magento's `eav_attribute` table with `entity_type_code = 'customer'`
- Most standard customer fields are flat columns on `customer_entity`, not EAV
