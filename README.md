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

`sort_by` is restricted to an allow-list
(`transaction_date, amount, created_at, category, status`) — a
client can't inject an arbitrary column into `ORDER BY`.

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
