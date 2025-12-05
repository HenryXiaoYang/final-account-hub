# Account Hub

A self-hosted account management system with API access, validation scripts, and a web dashboard.

## Purpose

Account Hub helps you manage pools of accounts (credentials, tokens, API keys, etc.) with:

- **Categorized storage** - Organize accounts into categories
- **Fetch & mark used** - API to fetch available accounts and automatically mark them as used
- **Validation scripts** - Python scripts to validate accounts on a schedule (check if banned, expired, etc.)
- **Web dashboard** - View stats, manage accounts, and configure validation
- **API history** - Track API calls with request IP logging

## Deploy

### Docker (Recommended)

```bash
# Clone and configure
cp .env.example .env
# Edit .env with your settings

# Run
docker compose up -d
```

### Manual

```bash
# Build frontend
cd frontend && npm ci && npm run build && cd ..

# Build backend
go build -o account-hub .

# Run
./account-hub
```

## Configuration

Environment variables (`.env`):

```env
PASSKEY=your-secure-passkey-here
PORT=8080
GIN_MODE=release

# Database: sqlite (default) or postgres
DB_TYPE=sqlite
DATABASE_URL=postgres://user:pass@localhost:5432/dbname?sslmode=disable

# Rate limiting
RATE_LIMIT_MAX_ATTEMPTS=5
RATE_LIMIT_BLOCK_MINUTES=15

# Connection pool
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100
DB_CONN_MAX_LIFETIME_MINUTES=60
```

## API Usage

All API requests require the `X-Passkey` header.

### Create Category

```bash
curl -X POST http://localhost:8080/api/categories/ensure \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-accounts"}'
```

### Add Accounts

```bash
# Single
curl -X POST http://localhost:8080/api/accounts \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": "user:pass"}'

# Bulk
curl -X POST http://localhost:8080/api/accounts/bulk \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": ["user1:pass1", "user2:pass2"]}'
```

### Fetch Accounts

Fetches available accounts and marks them as used:

```bash
curl -X POST http://localhost:8080/api/accounts/fetch \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "count": 1}'
```

### Update Account Status

```bash
curl -X PUT http://localhost:8080/api/accounts/update \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"ids": [1, 2], "banned": true}'
```

### Health Check

```bash
curl http://localhost:8080/health
```

## Validation Scripts

Write Python scripts in the dashboard to validate accounts. The script must define:

```python
def validate(account: str) -> tuple[bool, bool]:
    # Return (used, banned)
    # Example: check if account is still valid
    return False, False
```

Install Python packages via the dashboard's UV package manager.
