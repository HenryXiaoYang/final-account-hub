# Final Account Hub

[English](README.md) | 简体中文

自托管的账号池管理系统，提供 RESTful API、自动化 Python 验证和 Web 管理面板。基于 Go (Gin) 和 React (TypeScript) 构建。

## 概述

Final Account Hub 用于管理各类账号池 -- 凭证、令牌、API 密钥或任何字符串形式的密钥 -- 按分类组织。系统提供原子性的获取并标记已用操作、基于 Python 脚本的定时验证（检测封禁或过期账号），以及功能完整的 Web 管理界面。

### 核心功能

- **分类存储** -- 将账号组织到命名分类中，每个分类独立配置
- **原子获取** -- `POST /api/accounts/fetch` 在单个事务中选取可用账号并标记为已用
- **自动验证** -- 每个分类可配置 Python 验证脚本，按 cron 表达式定时执行，支持并发控制和范围选择
- **隔离环境** -- 每个分类拥有独立的 Python 虚拟环境（由 `uv` 管理）
- **Web 面板** -- 实时统计、账号管理、验证监控和 API 参考文档
- **调用追踪** -- 完整的 API 请求日志，记录客户端 IP，每个分类可配置保留数量
- **双数据库支持** -- SQLite（默认，零配置）或 PostgreSQL（生产环境）

## 环境要求

- Go 1.25+
- Node.js 20+
- [uv](https://docs.astral.sh/uv/)（Python 包管理器，验证脚本运行所需）
- Python 3.12（Docker 环境中由 `uv` 自动安装）

## 部署

### Docker（推荐）

```bash
git clone https://github.com/HenryXiaoYang/final-account-hub.git
cd final-account-hub
cp .env.example .env
# 编辑 .env -- 至少设置 PASSKEY

docker compose up -d
```

容器内置 `uv` 和 Python 3.12。数据通过卷挂载持久化到 `./data/` 目录。

### 手动部署

```bash
# 构建前端
cd frontend && npm ci && npm run build && cd ..

# 构建后端
go build -o account-hub .

# 运行（确保已配置 .env 或导出环境变量）
./account-hub
```

服务器从 `./frontend/dist/` 提供前端 SPA，监听配置的端口。

## 配置

所有配置通过环境变量完成。在项目根目录创建 `.env` 文件：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PASSKEY` | *（必填）* | API 认证密钥，通过 `X-Passkey` 请求头传递 |
| `PORT` | `8080` | HTTP 监听端口 |
| `GIN_MODE` | `debug` | Gin 框架模式（`debug`、`release`、`test`） |
| `DB_TYPE` | `sqlite` | 数据库引擎：`sqlite` 或 `postgres` |
| `DATABASE_URL` | -- | PostgreSQL 连接字符串（仅 `DB_TYPE=postgres` 时使用） |
| `RATE_LIMIT_MAX_ATTEMPTS` | `5` | 认证失败次数上限，超过后封锁 IP |
| `RATE_LIMIT_BLOCK_MINUTES` | `15` | IP 封锁时长（分钟） |
| `DB_MAX_IDLE_CONNS` | `10` | 数据库连接池：最大空闲连接数 |
| `DB_MAX_OPEN_CONNS` | `100` | 数据库连接池：最大打开连接数 |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `60` | 数据库连接池：连接最大存活时间（分钟） |

## 架构

```
main.go                  入口，服务器启动，优雅关闭
database/
  db.go                  数据库初始化，自动迁移，连接池配置
  models.go              GORM 模型：Category, Account, ValidationRun, APICallHistory, AccountSnapshot
  snapshot.go            定时快照采集，用于趋势图表
handlers/
  account.go             账号增删改查、获取、批量更新、统计、快照
  category.go            分类管理、验证配置、包管理、概览
  history.go             API 调用历史、频率分析、健康检查
routes/routes.go         路由注册与认证中间件
validator/validator.go   Cron 调度器、Python 脚本执行、并发控制
middleware/auth.go       X-Passkey 认证与基于 IP 的速率限制
logger/                  结构化日志
frontend/                React + TypeScript + Vite + shadcn/ui + Recharts
```

### 数据库

GORM `AutoMigrate` 在每次启动时运行。新增的表、列和索引会自动创建，已有数据不会被删除或修改。从旧版本升级无需任何手动迁移操作。

### 验证引擎

每个分类可定义一个包含 `validate(account: str) -> tuple[bool, bool]` 函数的 Python 验证脚本。脚本还可以调用内置 helper `update_account(data="...")` 来改写当前账号数据。调度器的工作流程：

1. 读取分类配置中的 cron 表达式
2. 根据配置的范围（`available`、`used`、`banned` 或任意组合）选取账号
3. 以配置的并发数对每个账号执行脚本
4. 根据 `(used, banned)` 返回值更新账号状态，并应用 `update_account(data="...")` 的数据改写
5. 记录运行详情和日志

脚本在每个分类独立的虚拟环境中执行，路径为 `./data/venvs/{category_id}/`，由 `uv` 管理。可通过 Web 界面安装依赖或上传 `requirements.txt`。

## API 参考

`/api/*` 下的所有端点需要 `X-Passkey` 请求头。响应使用标准 HTTP 状态码和 JSON 格式。

### 认证

所有 `/api/*` 请求必须包含：

```
X-Passkey: YOUR_PASSKEY
```

认证失败按 IP 计数。超过 `RATE_LIMIT_MAX_ATTEMPTS` 次后，该 IP 将被封锁 `RATE_LIMIT_BLOCK_MINUTES` 分钟（返回 HTTP 429）。

### 健康检查

```
GET /health
```

```json
{"status": "ok"}
```

无需认证。

---

### 分类

#### 创建分类

```
POST /api/categories
```

```json
{"name": "my-accounts"}
```

响应 (201)：完整的分类对象。

#### 创建分类（幂等）

```
POST /api/categories/ensure
```

```json
{"name": "my-accounts"}
```

如果名称已存在则返回现有分类，否则创建新分类。

#### 分类列表

```
GET /api/categories
```

响应 (200)：分类对象数组，按 ID 排序。

#### 获取分类

```
GET /api/categories/:id
```

#### 删除分类

```
DELETE /api/categories/:id
```

级联删除：同时删除该分类下的所有账号、验证记录、API 历史和快照。

#### 分类概览（面板）

```
GET /api/categories/overview
```

响应 (200)：包含账号统计的分类数组：

```json
[{"id": 1, "name": "my-accounts", "total": 100, "available": 60, "used": 30, "banned": 10, "last_validated_at": "2025-01-01T12:00:00Z"}]
```

---

### 账号

#### 添加账号

```
POST /api/accounts
```

```json
{"category_id": 1, "data": "user:pass"}
```

响应 (201)：创建的账号对象。如果数据在该分类中已存在，返回 409。

#### 批量添加账号

```
POST /api/accounts/bulk
```

```json
{"category_id": 1, "data": ["user1:pass1", "user2:pass2", "user3:pass3"]}
```

响应 (201)：

```json
{"count": 3, "skipped": 0}
```

重复数据（请求内或与已有数据重复）会被静默跳过。每次请求最多 10,000 条。

#### 账号列表

```
GET /api/accounts/:category_id?page=1&limit=100
```

响应 (200)：

```json
{"data": [...], "total": 250, "page": 1, "limit": 100}
```

按 ID 排序。页码自动限制在有效范围内。limit 范围：1-1000，默认 100。

#### 获取账号

```
POST /api/accounts/fetch
```

```json
{"category_id": 1, "count": 5}
```

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `category_id` | number | *（必填）* | 目标分类 ID |
| `count` | number | *（必填）* | 获取账号数量（1-1000） |
| `order` | string | `"sequential"` | `"sequential"`（按 ID 升序）或 `"random"`（随机） |
| `account_type` | string \| string[] | `"available"` | 账号状态过滤。单个字符串或数组：`"available"`、`"used"`、`"banned"` |
| `mark_as_used` | boolean | `true` | 是否将获取的账号标记为已用 |
| `created_after` | string | -- | RFC 3339 时间戳，筛选此时间之后创建的账号 |
| `created_before` | string | -- | RFC 3339 时间戳，筛选此时间之前创建的账号 |
| `updated_after` | string | -- | RFC 3339 时间戳，筛选此时间之后更新的账号 |
| `updated_before` | string | -- | RFC 3339 时间戳，筛选此时间之前更新的账号 |

响应 (200)：账号对象数组。当 `mark_as_used` 为 true（默认）时，选中的账号在数据库事务中被原子性地标记为 `used`。此端点会记录到 API 调用历史。

账号类型说明：
- `"available"` -- 未使用且未封禁（`used=false, banned=false`）
- `"used"` -- 已使用但未封禁（`used=true, banned=false`）
- `"banned"` -- 已封禁，不论使用状态（`banned=true`）

示例：

```json
// 随机获取 5 个可用账号（默认行为，向后兼容）
{"category_id": 1, "count": 5, "order": "random"}

// 获取已用账号，不标记状态
{"category_id": 1, "count": 10, "account_type": "used", "mark_as_used": false}

// 获取最近 24 小时内创建的可用或已用账号
{"category_id": 1, "count": 20, "account_type": ["available", "used"], "created_after": "2025-01-01T00:00:00Z"}
```

#### 更新账号

```
PUT /api/accounts/:id
```

```json
{"data": "new-user:new-pass", "used": false, "banned": true}
```

所有字段可选，但至少提供一个。`data` 会检查分类内唯一性。响应 (200)：更新后的账号对象。

#### 批量更新账号

```
PUT /api/accounts/batch/update
```

```json
{"ids": [1, 2, 3], "used": false, "banned": true}
```

批量更新多个账号的状态字段。`used` 和 `banned` 至少提供一个。

#### 按条件删除账号

```
DELETE /api/accounts
```

```json
{"category_id": 1, "used": true, "banned": false}
```

通过 Server-Sent Events (SSE) 流式返回进度。每批删除 500 条。

#### 按 ID 删除账号

```
DELETE /api/accounts/by-ids
```

```json
{"ids": [1, 2, 3]}
```

每次请求最多 10,000 个 ID。

#### 账号统计

```
GET /api/accounts/:category_id/stats
```

响应 (200)：

```json
{"counts": {"total": 100, "available": 60, "used": 30, "banned": 10}}
```

#### 账号快照

```
GET /api/accounts/:category_id/snapshots?granularity=1d
```

粒度选项：`1h`、`1d`、`1w`。返回趋势图表的时序数据。

---

### 全局统计

#### 全局统计数据

```
GET /api/stats
```

响应 (200)：

```json
{"accounts": {"total": 500, "available": 300, "used": 150, "banned": 50}, "categories": 5}
```

#### 全局快照

```
GET /api/snapshots?granularity=1d
```

跨所有分类的聚合快照历史。

---

### 验证

#### 更新验证配置

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

scope 接受逗号分隔的值：`available`、`used`、`banned`。

#### 测试验证脚本

```
POST /api/categories/:id/test-validation
```

```json
{"script": "def validate(account: str) -> tuple[bool, bool]:\n    return False, False", "test_account": "user:pass"}
```

响应 (200)：

```json
{"success": true, "used": false, "banned": false}
```

30 秒超时。

#### 立即运行验证

```
POST /api/categories/:id/run-validation
```

在 cron 计划之外触发一次即时验证。

#### 停止验证

```
POST /api/categories/:id/stop-validation
```

通知正在运行的验证优雅停止。

#### 验证运行列表

```
GET /api/categories/:id/validation-runs?page=1&limit=20
```

响应 (200)：分页的验证运行列表，包含状态、计数和时间戳。

#### 获取验证运行日志

```
GET /api/validation-runs/:run_id/log?offset=0&limit=100
```

返回倒序排列的日志行，支持分页。

#### 最近验证运行（面板）

```
GET /api/validation-runs/recent?limit=10
```

返回所有分类中最近的验证运行，包含分类名称。

---

### Python 包管理

每个分类拥有独立的虚拟环境，通过 `uv` 管理包。

#### 包列表

```
GET /api/categories/:id/packages
```

#### 安装包

```
POST /api/categories/:id/packages/install
```

```json
{"package": "requests"}
```

#### 卸载包

```
POST /api/categories/:id/packages/uninstall
```

```json
{"package": "requests"}
```

#### 从 requirements.txt 安装

```
POST /api/categories/:id/packages/requirements
```

Multipart 表单上传，`file` 字段包含 `requirements.txt` 文件。

---

### API 调用历史

#### 历史列表

```
GET /api/categories/:id/history?page=1&limit=50
```

分页，按时间倒序排列。

#### 删除历史记录

```
DELETE /api/categories/:id/history
```

```json
{"ids": [1, 2, 3]}
```

#### 清空所有历史

```
DELETE /api/categories/:id/history/all
```

#### API 调用频率（面板）

```
GET /api/history/frequency?hours=24
```

响应 (200)：指定时间窗口内的每小时调用次数（最大 168 小时）。

```json
[{"hour": "2025-01-01 14:00", "count": 42}, {"hour": "2025-01-01 15:00", "count": 17}]
```

---

### 历史限制

#### 更新验证历史限制

```
PUT /api/categories/:id/validation-history-limit
```

```json
{"validation_history_limit": 100}
```

#### 更新 API 历史限制

```
PUT /api/categories/:id/api-history-limit
```

```json
{"api_history_limit": 2000}
```

## 验证脚本参考

每个分类可定义一个 Python 验证脚本，必须包含以下签名的 `validate` 函数：

```python
def validate(account: str) -> tuple[bool, bool]:
    """
    验证单个账号。

    参数：
        account: 数据库中存储的原始账号数据字符串。

    返回：
        (used, banned) 元组：
        - (False, False) -- 账号可用
        - (True, False)  -- 账号已用但未封禁
        - (False, True)  -- 账号已封禁
        - (True, True)   -- 账号已用且已封禁
    """
    # 示例：基于 HTTP 的凭证检查
    import requests
    username, password = account.split(":")
    resp = requests.post("https://api.example.com/login",
                         json={"user": username, "pass": password})
    if resp.status_code == 403:
        return False, True   # 已封禁
    if resp.status_code == 200:
        return False, False  # 仍然可用
    return True, False       # 已用 / 无效
```

在 `validate` 内，你也可以按需改写当前账号数据：

```python
def validate(account: str) -> tuple[bool, bool]:
    refreshed = refresh_token(account)
    if refreshed != account:
        update_account(data=refreshed)
    return False, False
```

### 脚本执行细节

- 脚本在分类独立的虚拟环境中运行，路径为 `./data/venvs/{category_id}/`
- 如果虚拟环境不存在，回退使用 `uv run --isolated --no-project`
- 每个账号独立验证，按配置的并发数执行
- 验证脚本可调用 `update_account(data="...")` 或 `set_account_data("...")` 来改写当前账号保存的数据
- 测试运行有 30 秒超时；生产运行无单账号超时限制
- 每次验证的 stdout/stderr 输出会被捕获到运行日志中

## 许可证

MIT
