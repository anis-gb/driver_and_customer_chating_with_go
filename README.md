# go-starter

A standard-layout Go REST API starter using [chi](https://github.com/go-chi/chi) as the
router and [pgx](https://github.com/jackc/pgx) for PostgreSQL access.

## Folder structure

```
go-starter/
├── cmd/
│   └── api/            # main.go — application entrypoint
├── internal/
│   ├── config/          # env/config loading
│   ├── database/         # PostgreSQL connection pool
│   ├── handler/          # HTTP handlers (controllers)
│   ├── middleware/        # HTTP middleware (logging, etc.)
│   └── router/           # route registration
├── pkg/
│   └── response/          # shared reusable packages (JSON envelope)
├── migrations/            # raw SQL migrations
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

This follows the community-standard [golang-standards/project-layout](https://github.com/golang-standards/project-layout)
conventions: `cmd/` for entrypoints, `internal/` for private application code
that can't be imported by other projects, and `pkg/` for code that's safe to
share/reuse.

## Prerequisites

- Go 1.22+
- PostgreSQL running locally (or a connection string to a remote instance)

## Setup

1. Copy the example env file and adjust values:

   ```bash
   cp .env.example .env
   ```

2. Create the database referenced in `DATABASE_URL` (default name: `go_starter`):

   ```bash
   createdb go_starter
   ```

3. Install dependencies:

   ```bash
   make tidy
   ```

4. (Optional) apply the sample migration:

   ```bash
   make migrate-up
   ```

5. Run the API:

   ```bash
   make run
   ```

   The server starts on `http://localhost:8080` by default.

## Endpoints

| Method | Path              | Description                          |
|--------|-------------------|---------------------------------------|
| GET    | `/api/v1/hello`   | Returns a "hello world" JSON message |
| GET    | `/api/v1/health`  | Health check, verifies DB connectivity |

### Try it

```bash
curl http://localhost:8080/api/v1/hello
```

Response:

```json
{
  "success": true,
  "message": "hello world"
}
```

## Next steps

- Add more resources by creating a handler in `internal/handler/`, wiring it
  in `internal/router/router.go`, and adding any queries in `internal/database/`
  or a new `internal/repository/` package as the app grows.
- Swap the manual SQL migration approach for a tool like
  [golang-migrate](https://github.com/golang-migrate/migrate) once you have
  more than a couple of migrations.
- Add unit tests alongside each package (`*_test.go`).
