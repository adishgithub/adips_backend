# adips_backend

Gin + GORM + PostgreSQL backend, restructured into a layered
architecture.

## Architecture

```
cmd/api/main.go          Entrypoint. Loads config, connects DB, wires every
                          layer explicitly, starts the server with graceful
                          shutdown on SIGINT/SIGTERM.

config/                  Typed env config (config.Load()), validated once
                          at startup instead of os.Getenv() scattered
                          around the codebase.

internal/models/         GORM models (DB shape only — no JSON-facing
                          concerns beyond struct tags).

internal/dto/             Request/response structs. Keeps the public API
                          contract decoupled from the DB schema, and lets
                          gin's binding validation (`binding:"required"`
                          etc.) replace manual if-empty checks.

internal/repository/      One interface + one GORM implementation per
                          resource. Only layer that touches *gorm.DB.

internal/service/         Business logic: validation, ownership checks,
                          orchestration. Returns *utils.AppError, never
                          writes HTTP responses directly — this is what
                          makes it unit-testable without gin.Context.

internal/handler/         Thin Gin handlers: parse request -> call service
                          -> utils.RespondError or utils.Ok/Created.

internal/middleware/      Auth middleware, constructed with its
                          dependencies (jwt.Manager, UserRepository)
                          instead of reading globals.

internal/routes/          Single place routes are registered, versioned
                          under /api/v1.

internal/utils/           Response envelope, pagination, sort, search,
                          and the AppError type shared across layers.

pkg/jwt/                  Token generation/parsing, used by both the auth
                          service (issues) and the auth middleware
                          (validates).
```

Request flow: `handler` parses/validates the HTTP request → calls
`service` → `service` calls `repository` → `repository` talks to
Postgres via GORM. Each layer only knows about the one below it.

## Why this shape

- **Testability** — services depend on repository *interfaces*, so
  business logic can be unit tested with an in-memory fake, no DB
  required.
- **Security** — every transaction endpoint scopes to the user ID
  taken from the verified JWT (`internal/middleware` → `context`),
  never from a client-supplied `user_id`. The old `GetTransactions`
  trusted a `?user_id=` query param, which meant any authenticated
  user could read anyone else's transactions by changing the
  parameter — that's fixed here.
- **Consistent errors** — services return `*utils.AppError{Status,
  Message}`; handlers call `utils.RespondError(c, err)` once. No more
  hand-rolled `c.JSON(http.StatusX, ...)` repeated in every branch.
- **No hidden globals** — the old code used a package-level `DB`
  variable populated by an `init()` side effect and read `os.Getenv`
  directly inside `middleware.RequireAuth`. Everything is now
  constructed once in `main.go` and passed down explicitly.

## Setup

```bash
cp .env.example .env   # fill in DB / JWT_SECRET
go mod tidy            # re-resolve go.sum for the new import paths
go build ./...
go run ./cmd/api
```

## API

All routes are under `/api/v1` except the health check.

### Health

| Method | Path        |
|--------|-------------|
| GET    | `/healthz`  |

### Users (`/api/v1/users`)

| Method | Path        | Auth | Description        |
|--------|-------------|------|---------------------|
| POST   | `/signup`   | No   | Create an account   |
| POST   | `/login`    | No   | Get a JWT           |
| POST   | `/logout`   | No   | Clear auth cookie   |
| GET    | `/validate` | Yes  | Confirm token/session |

### Transactions (`/api/v1/transactions`) — all require auth, all scoped to the caller

| Method | Path        | Description                                    | Status |
|--------|-------------|-------------------------------------------------|--------|
| POST   | ``          | Create a transaction                             | done (already existed) |
| GET    | ``          | List, with filters + search + sort + pagination  | done (already existed) |
| GET    | `/summary`  | Aggregate totals (credit/debit/balance/count)    | **new** |
| GET    | `/:id`      | Get a single transaction                         | **new — was a commented-out stub** |
| PATCH  | `/:id`      | Partial update (only sent fields change)         | **new — was a commented-out stub** |
| DELETE | `/:id`      | Delete a transaction                             | **new — was a commented-out stub** |

`GET /transactions` and `GET /transactions/summary` accept the same
query params:

```
type, category, status, payment_method, currency,
min_amount, max_amount, start_date, end_date, search,
sort_by, order, page, limit
```

**Summary status rule:** `GET /transactions/summary` counts only
`completed` transactions when no `status` is sent (failed transactions
never happened and pending ones have not settled). Send
`status=pending` or `status=failed` to total those instead, or
`status=all` to count every status. `GET /transactions` (the list) is
unchanged and still returns all statuses by default.

**Transfers in transactions (Phase 2):** transfer legs appear in
`GET /transactions` (they are real rows that move balances) and carry a
`transfer_group_id`. `GET /transactions/summary` **excludes** them by
default because moving your own money is neither income nor expense;
send `include_transfers=true` to count them. A leg cannot be edited or
deleted through `/transactions/:id` (409); use `/transfers/:group_id`.

**Archived accounts are read-only:** creating, editing, moving or
deleting a transaction on an archived account returns 400. Unarchive
the account first.

**Money precision:** `transactions.amount` is stored as
`numeric(14,2)` (max `999,999,999,999.99`), so sums are exact in
Postgres. On startup, `database.Migrate` converts an existing
`double precision` column automatically (idempotent, logged, rounds to
2 decimals). Back up the database before the first deploy of this
change.

`sort_by` is restricted to an allow-list
(`transaction_date, amount, created_at, category, status`) — a
client can't inject an arbitrary column into `ORDER BY`.

### Accounts (`/api/v1/accounts`) — all require auth, all scoped to the caller

| Method | Path                    | Description |
|--------|-------------------------|-------------|
| POST   | ``                      | Create an account. `include_in_total` defaults to `true` when omitted |
| GET    | ``                      | List (archived hidden unless `include_archived=true`) with `current_balance` |
| GET    | `/summary`              | Dashboard totals (active accounts only) |
| GET    | `/:id`                  | Get one account |
| PATCH  | `/:id`                  | Partial update (an archived account cannot become the default) |
| PATCH  | `/reorder`              | Body `{"items":[{"id":1,"sort_order":0}]}`; one bad id fails the whole batch (400) |
| PATCH  | `/:id/archive`          | Hide an account (idempotent). 409 if default (A6), last active (A7) or balance ≠ 0 (A11) |
| PATCH  | `/:id/unarchive`        | Always allowed (idempotent) |
| GET    | `/:id/delete-preview?move_to=<id>` | Read-only preview of a merge-delete |
| DELETE | `/:id`                  | Delete. No transactions: soft delete (A8). With transactions: 409 (A9) unless `?move_transactions_to=<id>` |
| POST   | `/:id/adjust`           | Body `{"actual_balance": 0, "note": ""}`; creates one `Balance Adjustment` transaction, or nothing if already correct |

**Balances are derived, never stored:** `current_balance` = opening
balance + completed credits − completed debits dated up to now.
Negative balances are allowed.

**Merge-delete** (`DELETE /:id?move_transactions_to=<id>`) runs in one
DB transaction: the source's transactions move to the target, transfers
between the two accounts are collapsed (their net effect is zero), the
source's opening balance is added to the target's, and the source is
soft-deleted. The sum of all balances never changes, so
`target_balance_after = target_balance_before + source_current_balance`,
which is exactly what `delete-preview` returns. The target must be
active, owned by the caller and use the same currency. The source must
not be the default account or the last active account.

`delete-preview` returns `moved_transaction_count` (kept and re-pointed;
legs of collapsed transfers are not counted), `collapsed_transfer_count`,
`source_current_balance`, `target_balance_before`, `target_balance_after`.

`DELETE` and `PATCH /reorder` answer `200` with a message and no `data`.

### Transfers (`/api/v1/transfers`) — all require auth

| Method | Path          | Description |
|--------|---------------|-------------|
| POST   | ``            | `{from_account_id, to_account_id, amount, transaction_date?, note?}` |
| GET    | `/:group_id`  | Both legs: `{transfer_group_id, debit, credit}` |
| PATCH  | `/:group_id`  | Change `from_account_id`, `to_account_id`, `amount`, `transaction_date`, `note` on both legs atomically |
| DELETE | `/:group_id`  | Delete both legs |

A transfer is two ordinary transactions (a debit on the source, a
credit on the destination) sharing one `transfer_group_id`, both
`completed`, category `Transfer` (icon 30, color 1). Rules: the two
accounts must differ (X1), belong to the caller and be active (X2),
share one currency (X3), and `amount` must be > 0 after rounding to 2
decimals (X4). There is no insufficient-funds check (D5). An invalid
`group_id` is 400; another user's transfer is 404.

## Testing

`go test ./...` runs the unit tests. The database integration tests
(transfers, archive, delete, merge-delete, adjust) run the real
repositories against real PostgreSQL and are **skipped** unless
`TEST_DATABASE_URL` is set:

```
createdb adips_test
TEST_DATABASE_URL="postgres://USER:PASS@localhost:5432/adips_test?sslmode=disable" \
    go test ./internal/service/ -v
```

They **truncate every table**, so the database name must end in
`_test` or they refuse to run. Never point them at a real database.

## Notes / follow-ups worth doing next

- Add integration tests against a test Postgres instance (or
  sqlite for speed) covering the repository layer.
- Add request-ID + structured logging middleware for production
  observability.
- Add rate limiting on `/login` and `/signup`.
- Consider soft-delete-aware unique constraints (GORM's default
  `gorm.Model` soft-delete means a re-signup with a previously
  deleted email currently gets a uniqueness conflict — usually fine,
  but worth a conscious decision).
- `go.sum` was not regenerated in this sandbox (no network access to
  the Go module proxy here) — run `go mod tidy` after unzipping.