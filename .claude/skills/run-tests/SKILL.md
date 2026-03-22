---
name: run-tests
description: Run integration and comparison tests against the Magento database. Use when verifying changes work correctly.
argument-hint: [test-pattern]
disable-model-invocation: true
---

Run the project test suites. If a test pattern is provided, run only matching tests.

## Environment

Tests require these env vars (with defaults):
- `TEST_DB_HOST` (localhost)
- `TEST_DB_PORT` (3306)
- `TEST_DB_USER` (root)
- `TEST_DB_PASSWORD` ("")
- `TEST_DB_NAME` (magento)
- `GO_GRAPHQL_URL` (http://localhost:8082/graphql) — for comparison tests
- `MAGE_GRAPHQL_URL` (http://localhost:8080/graphql) — for comparison tests

## Steps

### 1. Build check

```bash
go build ./...
go vet ./...
```

### 2. Integration tests

If `$ARGUMENTS` is provided, use it as the test pattern:

```bash
go test ./tests/ -run '$ARGUMENTS' -v -timeout 60s -count=1
```

If no argument, run all integration tests:

```bash
go test ./tests/ -v -timeout 60s -count=1
```

### 3. Comparison tests (only if Go service is running)

Check if the Go service is running at `GO_GRAPHQL_URL`:

```bash
curl -sf http://localhost:8082/health > /dev/null 2>&1
```

If healthy, run comparison tests:

```bash
go test ./tests/ -run TestCompare -v -timeout 300s -count=1
```

### 4. Report results

Summarize pass/fail counts. For failures, show the test name and error message.
