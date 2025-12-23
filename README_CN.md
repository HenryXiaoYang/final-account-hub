# 🔐 Account Hub

[English](README.md) | 简体中文

一个自托管的账号管理系统，支持 API 访问、验证脚本和 Web 仪表盘。

## ✨ 功能特点

Account Hub 帮助您管理账号池（凭证、令牌、API 密钥等）：

- **📁 分类存储** - 将账号组织到不同分类中
- **🔄 获取并标记已用** - API 获取可用账号并自动标记为已使用
- **✅ 验证脚本** - Python 脚本定时验证账号（检查是否被封禁、过期等）
- **📊 Web 仪表盘** - 查看统计数据、管理账号、配置验证
- **📜 API 历史** - 记录 API 调用及请求 IP

## 🚀 部署

### Docker（推荐）

```bash
# 克隆并配置
cp .env.example .env
# 编辑 .env 设置

# 运行
docker compose up -d
```

### 手动部署

```bash
# 构建前端
cd frontend && npm ci && npm run build && cd ..

# 构建后端
go build -o account-hub .

# 运行
./account-hub
```

## ⚙️ 配置

环境变量（`.env`）：

```env
PASSKEY=your-secure-passkey-here
PORT=8080
GIN_MODE=release

# 数据库：sqlite（默认）或 postgres
DB_TYPE=sqlite
DATABASE_URL=postgres://user:pass@localhost:5432/dbname?sslmode=disable

# 速率限制
RATE_LIMIT_MAX_ATTEMPTS=5
RATE_LIMIT_BLOCK_MINUTES=15

# 连接池
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100
DB_CONN_MAX_LIFETIME_MINUTES=60
```

## 🖥️ Web 仪表盘

### 首页

未选择分类时显示全局概览：
- **统计卡片**：分类数量、可用/已用/已封禁账号数
- **数据图表**：折线图展示每日新增、可用、已用、已封禁账号趋势（跨所有分类）
- **API 参考**：常用 API 示例

### 账号标签页

- **统计图表**：折线图展示新增、可用、已用、已封禁账号随时间变化
- **添加账号**：粘贴账号数据（每行一个可批量导入）
- **账号表格**：查看、选择和管理账号，带状态标签
- **批量操作**：将选中账号设为已用/可用/已封禁，或删除

### 验证标签页

#### 验证脚本

编写 Python 函数验证账号。脚本必须定义：

```python
def validate(account: str) -> tuple[bool, bool]:
    """
    验证账号并返回其状态。

    参数：
        account: 账号数据字符串（如 "user:pass" 或 JSON）

    返回：
        tuple[bool, bool]: (used, banned)
        - (False, False) = 账号可用
        - (True, False) = 账号已用但未封禁
        - (False, True) = 账号已封禁
        - (True, True) = 账号已用且已封禁
    """
    # 示例：检查账号凭证是否仍然有效
    username, password = account.split(":")
    # ... 你的验证逻辑 ...
    return False, False  # 账号可用
```

**配置选项：**
- **Cron 表达式**：设置验证运行时间（如 `0 0 * * *` 每天午夜）
- **并发数**：同时验证的账号数量
- **立即运行**：手动触发验证
- **测试脚本**：用示例账号测试脚本后再运行全部

#### Python 依赖

安装验证脚本所需的包：

1. 在输入框中输入包名（如 `requests`、`httpx`）
2. 点击播放按钮安装
3. 或上传 `requirements.txt` 文件批量安装
4. 在下方表格查看已安装的包
5. 选择并删除不再需要的包

每个分类有独立的 Python 虚拟环境。

#### 运行历史

查看历史验证运行：
- 开始时间和结束时间
- 状态（运行中/成功/失败）
- 处理的账号总数和封禁数
- 点击日志图标查看详细执行日志

### API 标签页

#### API 示例

仪表盘显示常用操作的 `curl` 命令：
- 添加单个账号
- 获取可用账号（标记为已用）
- 标记账号为已封禁

复制并修改这些示例用于您的集成。

#### API 调用历史

跟踪对此分类的所有 API 调用：
- 每次调用的时间戳
- HTTP 方法和端点
- 响应状态码
- 请求体
- 客户端 IP 地址

**历史限制**：配置保留多少条 API 调用记录（默认：1000）。旧记录会自动删除。

## 📡 API 使用

所有 API 请求需要 `X-Passkey` 请求头。

### 创建分类

```bash
curl -X POST http://localhost:8080/api/categories/ensure \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-accounts"}'
```

### 添加账号

```bash
# 单个
curl -X POST http://localhost:8080/api/accounts \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": "user:pass"}'

# 批量
curl -X POST http://localhost:8080/api/accounts/bulk \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": ["user1:pass1", "user2:pass2"]}'
```

### 获取账号

获取可用账号并标记为已用：

```bash
curl -X POST http://localhost:8080/api/accounts/fetch \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "count": 1}'
```

### 更新账号状态

```bash
curl -X PUT http://localhost:8080/api/accounts/update \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"ids": [1, 2], "banned": true}'
```

### 删除账号

```bash
# 删除已用账号
curl -X DELETE "http://localhost:8080/api/accounts?category_id=1&used=true" \
  -H "X-Passkey: YOUR_PASSKEY"

# 删除已封禁账号
curl -X DELETE "http://localhost:8080/api/accounts?category_id=1&banned=true" \
  -H "X-Passkey: YOUR_PASSKEY"
```

### 健康检查

```bash
curl http://localhost:8080/health
```
