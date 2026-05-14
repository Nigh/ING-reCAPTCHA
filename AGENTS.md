# AGENTS.md

## Project Overview

A Telegram bot for Ingress-themed group verification. New members must pass an image-recognition CAPTCHA (composed from labeled image sets) before gaining speaking permissions. The bot handles the full lifecycle: restrict on join, present challenge, evaluate answers, kick/ban on failure.

## Tech Stack

- **Language:** Go 1.21 (CGO required for SQLite)
- **Database:** SQLite via `go-sqlite3`, WAL mode, in-process migrations
- **Telegram API:** `go-telegram-bot-api/telegram-bot-api/v5`
- **Image processing:** `disintegration/imaging`
- **Testing:** `testify` (assert + require), in-memory SQLite (`:memory:`)
- **i18n:** Embedded JSON locale files (`internal/i18n/locales/`)
- **Deployment:** Docker (multi-stage build), docker-compose

## Repository Structure

```
cmd/bot/                    # Application entrypoint (main.go)
internal/
  bot/                      # Telegram bot core: handlers, verification flow, admin commands, i18n bridge
  config/                   # Environment-based config loading
  database/                 # SQLite DB layer: models, queries, inline migrations
  i18n/                     # Translator with embedded locale JSON files
    locales/                # en.json, zh.json (must have identical key sets)
  imaging/                  # Image composition with noise/interference for anti-hash-scan
migrations/                 # Historical SQL migrations (superseded by inline migration in db.go)
.env.example                # Reference for all environment variables
Dockerfile                  # Multi-stage build (golang:1.21-alpine -> alpine:3.19)
docker-compose.yml          # Single-service bot with volume mounts for data/ and images/
```

## Development Commands

```bash
# Install dependencies
go mod download

# Build (CGO required)
CGO_ENABLED=1 go build -o bot ./cmd/bot

# Run
go run ./cmd/bot

# Test all packages
go test ./...

# Test a specific package
go test ./internal/bot/...
go test ./internal/database/...

# Format
go fmt ./...

# Vet
go vet ./...
```

### Environment Setup

```bash
cp .env.example .env
# Edit .env with at minimum TELEGRAM_BOT_TOKEN and BOT_ADMIN_IDS
```

## Coding Conventions

### Structure & Style
- Standard Go layout: `cmd/` for entrypoints, `internal/` for private packages
- No comments in code unless explaining non-obvious business logic
- Use `fmt.Errorf("...: %w", err)` for error wrapping throughout
- `log.Printf` for logging; prefixed with `[DEBUG]`, `[INFO]`, `[WARN]`, `[ERROR]` in bot package
- Structs use exported fields for domain models, unexported for internal state

### Dependency Injection
- `TelegramAPI` interface in `internal/bot/telegram_api.go` abstracts `*tgbotapi.BotAPI` for testability
- Tests inject mocks via this interface; new Telegram API methods must be added to the interface

### Database
- All DB access through `database.DB` methods (wraps `*sql.DB`)
- Migrations run inline in `db.go:migrateFromFile()` using `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`
- The `migrations/` directory contains historical migrations only; the source of truth is the inline SQL in `db.go`
- Use parameterized queries (`?`), never string concatenation for SQL
- SQLite connection string includes `?_foreign_keys=on&_busy_timeout=5000`

### i18n
- All user-facing strings use the `b.t(user, "key", args...)` pattern
- Locale files in `internal/i18n/locales/*.json` must have identical key sets across all languages
- Adding a new language: create a new JSON file with all existing keys
- Adding a new string: add the key to **all** locale files

### Configuration
- All config via environment variables, loaded in `internal/config/config.go`
- Every env var has a sensible default except `TELEGRAM_BOT_TOKEN`

## Testing Guidelines

- Tests live alongside source files (`*_test.go`)
- Database tests use `:memory:` SQLite via `newTestDB(t)` helper (see `internal/database/db_test.go`)
- Bot tests use `newTestBot(t)` helper with mock `TelegramAPI` (see `internal/bot/bot_core_test.go`)
- Use `require` for fatal assertions, `assert` for non-fatal
- Use `t.Run` for subtests
- Set `db.SetMaxOpenConns(1)` in tests to avoid SQLite locking issues with `:memory:` databases

## Database Tables

| Table | Purpose |
|-------|---------|
| `image_sets` | Named categories of verification images |
| `images` | Individual images belonging to a set (Telegram file_id + local cache path) |
| `image_set_labels` | Multilingual labels for image sets |
| `pending_verifications` | Active verification sessions (per chat+user, UPSERT on conflict) |
| `verification_failures` | Tracks failure count for two-strike kick/ban logic |
| `user_join_history` | Anti-spam: recent join timestamps |
| `user_language_preferences` | Per-user language override |
| `admins` | Bot-level admin user IDs |

## Contribution Rules for Agents

- Make minimal, focused changes. Do not refactor unrelated code.
- Preserve existing code style, naming, and architectural patterns.
- Run `go vet ./...` and `go test ./...` before finishing.
- When adding i18n keys, update **all** locale JSON files in `internal/i18n/locales/`.
- When adding new database tables or columns, update the inline migration in `internal/database/db.go` (not the `migrations/` directory).
- When changing user-facing behavior, verify all affected bot commands still function.
- Update this AGENTS.md whenever repository conventions, commands, structure, or workflows change.
