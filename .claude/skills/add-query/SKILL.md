---
name: add-query
description: Add a new root-level GraphQL query or mutation. Use when extending the API beyond customer().
argument-hint: <query_name>
---

Add a new root-level GraphQL query or mutation `$ARGUMENTS`. Follow these steps:

## 1. Schema

In `graph/schema.graphqls`, add the query/mutation to the `Query` or `Mutation` type and define all needed types.

## 2. Regenerate

```bash
GOTOOLCHAIN=auto go run github.com/99designs/gqlgen generate
```

This creates a resolver stub in `graph/schema.resolvers.go`.

## 3. Repository

Create `internal/repository/<domain>.go`:
- Struct with `*sql.DB` field
- Constructor: `NewXxxRepository(db *sql.DB) *XxxRepository`
- Query methods that accept `ctx context.Context` as first parameter
- Customer entity uses `entity_id` (NOT `row_id`)

## 4. Service (optional)

If business logic is needed beyond simple data fetching, add methods to `internal/service/customer.go` or create a new service file.

## 5. Wiring

In `graph/resolver.go`:
- Add the new repository/service to the `Resolver` struct
- Instantiate it in `NewResolver()`

## 6. Resolver

In `graph/schema.resolvers.go`, implement the generated stub by calling your service/repository.

For authenticated queries/mutations, check `middleware.GetCustomerID(ctx)` and return an error if 0.

## 7. Verify

```bash
GOTOOLCHAIN=auto go build ./...
GOTOOLCHAIN=auto go vet ./...
```
