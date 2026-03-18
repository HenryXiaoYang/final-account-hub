# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Backend
go build -o account-hub .
./account-hub

# Frontend
cd frontend && npm ci && npm run build

# Development (frontend hot reload)
cd frontend && npm run dev

# Docker
docker compose up -d
```

## Architecture

Go backend (Gin) + Vue 3 frontend (Vite + PrimeVue + Tailwind).

### Backend Structure
- `main.go` - Entry point, initializes DB, validator scheduler, routes, serves static frontend
- `database/` - GORM models (Category, Account, ValidationRun, APICallHistory) and DB init
- `handlers/` - HTTP handlers for accounts, categories, history, validation
- `routes/routes.go` - All API routes under `/api` with auth middleware
- `validator/validator.go` - Cron-based validation scheduler, runs Python scripts per category
- `middleware/auth.go` - X-Passkey header authentication
- `logger/` - Custom logging

### Frontend Structure (`frontend/src/`)
- Vue 3 SPA with vue-router
- `views/` - Dashboard, category management, validation UI
- `api/` - Axios API client
- `i18n/` - Internationalization
- Monaco editor for Python validation scripts

### Key Flows
- **Account fetch**: `POST /api/accounts/fetch` atomically selects available accounts and marks them used
- **Validation**: Python scripts run via `uv` (or category venv), return `(used, banned)` tuple. Scheduled via cron expressions per category.
- **Venvs**: Per-category Python venvs stored in `./data/venvs/{category_id}/`

## Database

SQLite (default) or PostgreSQL. Models auto-migrate on startup. Configure via `DB_TYPE` and `DATABASE_URL` env vars.

## API Authentication

All `/api/*` routes require `X-Passkey` header matching `PASSKEY` env var.

## Core Instruction for CodeX MCP

At any moment, you must think about how the current process can collaborate with codex and how to invoke the MCP tools provided by Codex as a guarantee for your objective and comprehensive analysis.
Among these, you **must execute** the following steps:
**1** After forming a preliminary analysis of the user's requirements, inform codex of the user's needs and initial thoughts, and require it to refine the requirements analysis and implementation plan.
**2** Before implementing specific coding tasks, **you must request a code implementation prototype from codex (require codex to only provide unified diff patch, strictly prohibit making any actual code modifications)**. After obtaining the code prototype, you **can only use it as a logical reference, then rewrite the code modifications again**, forming enterprise production-level, highly readable, and highly maintainable code before implementing specific programming modification tasks.
**3** Whenever actual coding behavior is completed, **you must immediately use codex to review the code changes and the degree of corresponding requirement completion**.
**4** Codex can only provide references, you **must have your own thinking and even need to question codex's answers**. Blindly believing books is worse than having no books at all. Your and codex's ultimate mission is to achieve unified, comprehensive, and accurate opinions, so you must continuously debate to find the only path to truth.


## Codex Tool Invocation Specification

1. Tool Overview

codex MCP provides a tool `codex` for executing AI-assisted coding tasks. This tool **is invoked through the MCP protocol** and does not require command line usage.

2. Tool Parameters

**Required** parameters:
- PROMPT (string): Task instruction sent to codex
- cd (Path): Root path of the working directory where codex executes tasks

Optional parameters:
- sandbox (string): Sandbox policy, available values:
    - "read-only" (default): Read-only mode, most secure
    - "workspace-write": Allows writing in workspace
    - "danger-full-access": Full access permissions
- SESSION_ID (UUID | null): Used to continue a previous session for multi-turn interaction with codex, defaults to None (start new session)
- skip_git_repo_check (boolean): Whether to allow running in non-Git repositories, defaults to False
- return_all_messages (boolean): Whether to return all messages (including reasoning, tool calls, etc.), defaults to False
- image (List[Path] | null): Attach one or more image files to the initial prompt, defaults to None
- model (string | null): Specify the model to use, defaults to None (use user's default configuration)
- yolo (boolean | null): Run all commands without approval (skip sandbox), defaults to False
- profile (string | null): Configuration profile name loaded from `~/.codex/config.toml`, defaults to None (use user's default configuration)

Return value:
{
"success": true,
"SESSION_ID": "uuid-string",
"agent_messages": "text content of agent's reply",
"all_messages": []  // Only included when return_all_messages=True
}
Or on failure:
{
"success": false,
"error": "error message"
}

3. Usage

Starting a new conversation:
- Do not pass SESSION_ID parameter (or pass None)
- Tool will return a new SESSION_ID for subsequent conversations

Continuing a previous conversation:
- Pass the previously returned SESSION_ID as a parameter
- Context of the same session will be preserved

4. Invocation Specification

**Must comply**:
- Each time the codex tool is invoked, the returned SESSION_ID must be saved for subsequent conversation continuation
- The cd parameter must point to an existing directory, otherwise the tool will fail silently
- Strictly prohibit codex from making actual code modifications, use sandbox="read-only" to avoid accidents, and require codex to only provide unified diff patch

Recommended usage:
- If detailed tracking of codex's reasoning process and tool calls is needed, set return_all_messages=True
- For tasks such as precise positioning, debugging, and rapid code prototype writing, prioritize using the codex tool

5. Notes

- Session management: Always track SESSION_ID to avoid session confusion
- Working directory: Ensure the cd parameter points to the correct and existing directory
- Error handling: Check the success field of the return value and handle possible errors
