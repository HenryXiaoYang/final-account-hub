# Final Account Hub

English | [简体中文](README_CN.md)

A self-hosted account pool management system with RESTful API, automated Python validation, and a web dashboard. Built with Go (Gin) and React (TypeScript).

## Overview

Final Account Hub manages pools of accounts -- credentials, tokens, API keys, or any string-based secrets -- organized by category. It provides atomic fetch-and-mark-used operations for consumers, scheduled Python-based validation to detect banned or expired accounts, and a full-featured web UI for administration.

### Core Capabilities

- **Categorized storage** -- Organize accounts into named categories, each with independent configuration
- **Atomic fetch** -- `POST /api/accounts/fetch` selects available accounts and marks them used in a single transaction
- **Automated validation** -- Per-category Python scripts run on cron schedules with configurable concurrency and scope
- **Isolated environments** -- Each category gets its own Python virtual environment (managed by `uv`)
- **Web dashboard** -- Real-time statistics, account management, validation monitoring, and API reference
- **API call tracking** -- Full request logging with IP addresses, configurable retention per category
- **Dual database support** -- SQLite (default, zero-config) or PostgreSQL for production scale

## Requirements

- Go 1.25+
- Node.js 20+
- [uv](https://docs.astral.sh/uv/) (Python package manager, required for validation scripts)
- Python 3.12 (installed automatically by `uv` in Docker)

## Deployment

### Docker (Recommended)

```bash
git clone https://github.com/HenryXiaoYang/final-account-hub.git
cd final-account-hub
cp .env.example .env
# Edit .env -- at minimum, set PASSKEY

docker compose up -d
```

The container bundles `uv` and Python 3.12. Data is persisted to `./data/` via volume mount.

### Manual

```bash
# Build frontend
cd frontend && npm ci && npm run build && cd ..

# Build backend
go build -o account-hub .

# Run (ensure .env is configured or export environment variables)
./account-hub
```

The server serves the frontend SPA from `./frontend/dist/` and listens on the configured port.

## Configuration

All configuration is via environment variables. Create a `.env` file in the project root:

| Variable | Default | Description |
|---|---|---|
| `PASSKEY` | *(required)* | Shared secret for API authentication via `X-Passkey` header |
| `PORT` | `8080` | HTTP listen port |
| `GIN_MODE` | `debug` | Gin framework mode (`debug`, `release`, `test`) |
| `DB_TYPE` | `sqlite` | Database engine: `sqlite` or `postgres` |
| `DATABASE_URL` | -- | PostgreSQL connection string (only when `DB_TYPE=postgres`) |
| `RATE_LIMIT_MAX_ATTEMPTS` | `5` | Failed auth attempts before IP is blocked |
| `RATE_LIMIT_BLOCK_MINUTES` | `15` | Duration (minutes) an IP stays blocked after exceeding attempts |
| `DB_MAX_IDLE_CONNS` | `10` | Database connection pool: max idle connections |
| `DB_MAX_OPEN_CONNS` | `100` | Database connection pool: max open connections |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `60` | Database connection pool: max connection lifetime (minutes) |

## Architecture

```
main.go                  Entry point, server setup, graceful shutdown
database/
  db.go                  Database initialization, auto-migration, connection pool
  models.go              GORM models: Category, Account, ValidationRun, APICallHistory, AccountSnapshot
  snapshot.go            Periodic snapshot collection for trend charts
handlers/
  account.go             Account CRUD, fetch, batch update, stats, snapshots
  category.go            Category CRUD, validation config, package management, overview
  history.go             API call history, frequency analytics, health check
routes/routes.go         Route registration with auth middleware
validator/validator.go   Cron scheduler, Python script execution, concurrency control
middleware/auth.go       X-Passkey authentication with IP-based rate limiting
logger/                  Structured logging
frontend/                React + TypeScript + Vite + shadcn/ui + Recharts
```

### Database

GORM `AutoMigrate` runs on every startup. Schema changes (new tables, new columns, new indexes) are applied automatically. Existing data is never dropped or altered. Upgrading from an older version requires no manual migration steps.

### Validation Engine

Each category can define a Python validation script with a `validate(account: str) -> tuple[bool, bool]` function. The scheduler:

1. Reads the cron expression from the category configuration
2. Selects accounts matching the configured scope (`available`, `used`, `banned`, or any combination)
3. Runs the script against each account with the configured concurrency level
4. Updates account status based on the `(used, banned)` return value
5. Records the run with detailed logs

Scripts execute in per-category virtual environments at `./data/venvs/{category_id}/`, managed by `uv`. Dependencies can be installed through the web UI or via `requirements.txt` upload.

## API Reference

All endpoints under `/api/*` require the `X-Passkey` header. Responses use standard HTTP status codes with JSON bodies.

### Authentication

Every request to `/api/*` must include:

```
X-Passkey: YOUR_PASSKEY
```

Failed attempts are tracked per IP. After `RATE_LIMIT_MAX_ATTEMPTS` failures, the IP is blocked for `RATE_LIMIT_BLOCK_MINUTES` minutes (HTTP 429).

### Health Check

```
GET /health
```

```json
{"status": "ok"}
```

No authentication required.

---

### Categories

#### Create Category

```
POST /api/categories
```

```json
{"name": "my-accounts"}
```

Response (201):

```json
{"id": 1, "name": "my-accounts", "validation_script": "", "validation_concurrency": 1, "validation_cron": "0 0 * * *", "validation_history_limit": 50, "api_history_limit": 1000, "validation_enabled": true, "validation_scope": "available,used", "last_validated_at": null, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
```

#### Create Category (Idempotent)

```
POST /api/categories/ensure
```

```json
{"name": "my-accounts"}
```

Returns the existing category if the name already exists, or creates a new one.

#### List Categories

```
GET /api/categories
```

Response (200): Array of category objects, ordered by ID.

#### Get Category

```
GET /api/categories/:id
```

#### Delete Category

```
DELETE /api/categories/:id
```

Cascades: deletes all accounts, validation runs, API history, and snapshots for the category.

#### Categories Overview (Dashboard)

```
GET /api/categories/overview
```

Response (200): Array of categories with aggregated account counts:

```json
[{"id": 1, "name": "my-accounts", "total": 100, "available": 60, "used": 30, "banned": 10, "last_validated_at": "2025-01-01T12:00:00Z"}]
```

---

### Accounts

#### Add Account

```
POST /api/accounts
```

```json
{"category_id": 1, "data": "user:pass"}
```

Response (201): The created account object. Returns 409 if the data already exists in the category.

#### Add Accounts (Bulk)

```
POST /api/accounts/bulk
```

```json
{"category_id": 1, "data": ["user1:pass1", "user2:pass2", "user3:pass3"]}
```

Response (201):

```json
{"count": 3, "skipped": 0}
```

Duplicates (within the request or against existing data) are silently skipped. Maximum 10,000 items per request.

#### List Accounts

```
GET /api/accounts/:category_id?page=1&limit=100
```

Response (200):

```json
{"data": [...], "total": 250, "page": 1, "limit": 100}
```

Ordered by ID. Page is clamped to valid range. Limit range: 1-1000, default 100.

#### Fetch Accounts (Atomic)

```
POST /api/accounts/fetch
```

```json
{"category_id": 1, "count": 5}
```

Response (200): Array of account objects. Selected accounts are atomically marked as `used` within a database transaction. Count range: 1-1000. This endpoint is logged in API call history.

#### Update Account

```
PUT /api/accounts/:id
```

```json
{"data": "new-user:new-pass", "used": false, "banned": true}
```

All fields are optional, but at least one must be provided. `data` is checked for uniqueness within the category. Response (200): The updated account object.

#### Batch Update Accounts

```
PUT /api/accounts/batch/update
```

```json
{"ids": [1, 2, 3], "used": false, "banned": true}
```

Updates status fields for multiple accounts at once. At least one of `used` or `banned` must be provided.

#### Delete Accounts (by filter)

```
DELETE /api/accounts
```

```json
{"category_id": 1, "used": true, "banned": false}
```

Streams progress via Server-Sent Events (SSE). Deletes in batches of 500.

#### Delete Accounts (by IDs)

```
DELETE /api/accounts/by-ids
```

```json
{"ids": [1, 2, 3]}
```

Maximum 10,000 IDs per request.

#### Account Stats

```
GET /api/accounts/:category_id/stats
```

Response (200):

```json
{"counts": {"total": 100, "available": 60, "used": 30, "banned": 10}}
```

#### Account Snapshots

```
GET /api/accounts/:category_id/snapshots?granularity=1d
```

Granularity options: `1h`, `1d`, `1w`. Returns time-series data for trend charts.

---

### Global Statistics

#### Global Stats

```
GET /api/stats
```

Response (200):

```json
{"accounts": {"total": 500, "available": 300, "used": 150, "banned": 50}, "categories": 5}
```

#### Global Snapshots

```
GET /api/snapshots?granularity=1d
```

Aggregated snapshot history across all categories.

---

### Validation

#### Update Validation Configuration

```
PUT /api/categories/:id/validation-script
```

```json
{
  "validation_script": "def validate(account: str) -> tuple[bool, bool]:\n    return False, False",
  "validation_concurrency": 5,
  "validation_cron": "0 */6 * * *",
  "validation_enabled": true,
  "validation_scope": "available,used"
}
```

Scope accepts comma-separated values: `available`, `used`, `banned`.

#### Test Validation Script

```
POST /api/categories/:id/test-validation
```

```json
{"script": "def validate(account: str) -> tuple[bool, bool]:\n    return False, False", "test_account": "user:pass"}
```

Response (200):

```json
{"success": true, "used": false, "banned": false}
```

Runs with a 30-second timeout.

#### Run Validation Now

```
POST /api/categories/:id/run-validation
```

Triggers an immediate validation run outside the cron schedule.

#### Stop Validation

```
POST /api/categories/:id/stop-validation
```

Signals a running validation to stop gracefully.

#### List Validation Runs

```
GET /api/categories/:id/validation-runs?page=1&limit=20
```

Response (200): Paginated list of validation runs with status, counts, and timestamps.

#### Get Validation Run Log

```
GET /api/validation-runs/:run_id/log?offset=0&limit=100
```

Returns log lines in reverse chronological order with pagination support.

#### Recent Validation Runs (Dashboard)

```
GET /api/validation-runs/recent?limit=10
```

Returns the most recent validation runs across all categories, including category names.

---

### Python Package Management

Each category has an isolated virtual environment. Packages are managed via `uv`.

#### List Packages

```
GET /api/categories/:id/packages
```

#### Install Package

```
POST /api/categories/:id/packages/install
```

```json
{"package": "requests"}
```

#### Uninstall Package

```
POST /api/categories/:id/packages/uninstall
```

```json
{"package": "requests"}
```

#### Install from requirements.txt

```
POST /api/categories/:id/packages/requirements
```

Multipart form upload with a `file` field containing the `requirements.txt`.

---

### API Call History

#### List History

```
GET /api/categories/:id/history?page=1&limit=50
```

Paginated, ordered by most recent first.

#### Delete History Entries

```
DELETE /api/categories/:id/history
```

```json
{"ids": [1, 2, 3]}
```

#### Clear All History

```
DELETE /api/categories/:id/history/all
```

#### API Call Frequency (Dashboard)

```
GET /api/history/frequency?hours=24
```

Response (200): Hourly call counts for the specified time window (max 168 hours).

```json
[{"hour": "2025-01-01 14:00", "count": 42}, {"hour": "2025-01-01 15:00", "count": 17}]
```

---

### History Limits

#### Update Validation History Limit

```
PUT /api/categories/:id/validation-history-limit
```

```json
{"validation_history_limit": 100}
```

#### Update API History Limit

```
PUT /api/categories/:id/api-history-limit
```

```json
{"api_history_limit": 2000}
```

## Validation Script Reference

Each category can define a Python validation script. The script must contain a `validate` function with the following signature:

```python
def validate(account: str) -> tuple[bool, bool]:
    """
    Validate a single account.

    Args:
        account: The raw account data string as stored in the database.

    Returns:
        A tuple of (used, banned):
        - (False, False) -- account is available
        - (True, False)  -- account is used but not banned
        - (False, True)  -- account is banned
        - (True, True)   -- account is both used and banned
    """
    # Example: HTTP-based credential check
    import requests
    username, password = account.split(":")
    resp = requests.post("https://api.example.com/login",
                         json={"user": username, "pass": password})
    if resp.status_code == 403:
        return False, True   # banned
    if resp.status_code == 200:
        return False, False  # still available
    return True, False       # used / invalid
```

### Script Execution Details

- Scripts run in the category's isolated venv at `./data/venvs/{category_id}/`
- If no venv exists, `uv run --isolated --no-project` is used as fallback
- Each account is validated independently with the configured concurrency
- A 30-second timeout applies to test runs; production runs have no per-account timeout
- stdout/stderr from each validation is captured in the run log

## License

MIT
