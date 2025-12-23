# 🔐 Account Hub

English | [简体中文](README_CN.md)

A self-hosted account management system with API access, validation scripts, and a web dashboard.

## ✨ Purpose

Account Hub helps you manage pools of accounts (credentials, tokens, API keys, etc.) with:

- **📁 Categorized storage** - Organize accounts into categories
- **🔄 Fetch & mark used** - API to fetch available accounts and automatically mark them as used
- **✅ Validation scripts** - Python scripts to validate accounts on a schedule (check if banned, expired, etc.)
- **📊 Web dashboard** - View stats, manage accounts, and configure validation
- **📜 API history** - Track API calls with request IP logging

## 🚀 Deploy

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

## Web Dashboard

### Home Page

When no category is selected, the home page displays:
- **Statistics Cards**: Total categories, available/used/banned account counts
- **Data Graph**: Line chart showing Added, Available, Used, and Banned accounts over time across all categories
- **API Reference**: Common API examples for quick reference

### Accounts Tab

- **Statistics Chart**: Line chart showing Added, Available, Used, and Banned accounts over time
- **Add Accounts**: Paste account data (one per line for bulk import)
- **Account Table**: View, select, and manage accounts with status tags
- **Bulk Actions**: Set selected accounts as Used/Available/Banned, or delete

### Validation Tab

#### Validation Script

Write a Python function to validate accounts. The script must define:

```python
def validate(account: str) -> tuple[bool, bool]:
    """
    Validate an account and return its status.

    Args:
        account: The account data string (e.g., "user:pass" or JSON)

    Returns:
        tuple[bool, bool]: (used, banned)
        - (False, False) = Account is available
        - (True, False) = Account is used but not banned
        - (False, True) = Account is banned
        - (True, True) = Account is both used and banned
    """
    # Example: Check if account credentials are still valid
    username, password = account.split(":")
    # ... your validation logic here ...
    return False, False  # Account is available
```

**Configuration options:**
- **Cron Expression**: Schedule when validation runs (e.g., `0 0 * * *` for daily at midnight)
- **Concurrency**: Number of accounts to validate in parallel
- **Run Now**: Manually trigger validation immediately
- **Test Script**: Test your script with a sample account before running on all accounts

#### Python Dependencies

Install packages needed by your validation script:

1. Type package name in the input field (e.g., `requests`, `httpx`)
2. Click the play button to install
3. Or upload a `requirements.txt` file for bulk installation
4. View installed packages in the table below
5. Select and delete packages you no longer need

Each category has its own isolated Python virtual environment.

#### Run History

View past validation runs with:
- Start time and finish time
- Status (running/success/failed)
- Total accounts processed and banned count
- Click the log icon to view detailed execution logs

### API Tab

#### API Examples

The dashboard shows ready-to-use `curl` commands for common operations:
- Add single account
- Fetch available accounts (marks them as used)
- Mark accounts as banned

Copy and modify these examples for your integration.

#### API Call History

Track all API calls made to this category:
- Timestamp of each call
- HTTP method and endpoint
- Response status code
- Request body
- Client IP address

**History Limit**: Configure how many API calls to retain (default: 1000). Older entries are automatically deleted.

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

### Delete Accounts

```bash
# Delete used accounts
curl -X DELETE "http://localhost:8080/api/accounts?category_id=1&used=true" \
  -H "X-Passkey: YOUR_PASSKEY"

# Delete banned accounts
curl -X DELETE "http://localhost:8080/api/accounts?category_id=1&banned=true" \
  -H "X-Passkey: YOUR_PASSKEY"
```

### Health Check

```bash
curl http://localhost:8080/health
```
