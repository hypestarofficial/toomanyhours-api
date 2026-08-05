# toomanyhours-api

TooManyHours API — a Go REST API for the TooManyHours web application.

## Getting started

Requires Go 1.25.6+ and Docker.

```bash
# 1. Install the migration CLI (once)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 2. Start Postgres
docker compose up -d

# 3. Apply the schema
migrate -path migrations -database "$MIGRATE_DATABASE_URL" up

# 4. Load development seed data (optional, but the catalog is empty without it)
docker compose exec -T postgres psql -U toomanyhours -d toomanyhours < sql/seed.sql

# 5. Run the API
go run ./cmd/api
```

`./backup.sh` does steps 2 and 5 with a `pg_dump` into `backups/` in between, and is the usual day-to-day entrypoint.

### Environment

`.env` is gitignored. It needs:

| Variable | Notes |
|---|---|
| `VERSION`, `PORT` | API version string and listen port (default `3130`) |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Also read by `docker-compose.yml`, so changing them changes both the container and the connection string |
| `DB_SSLMODE`, `DB_TIMEZONE`, `DB_CONNECT_TIMEOUT` | Connection tuning |
| `JWT_SECRET` | Signing secret for both access and refresh tokens |
| `MIGRATE_DATABASE_URL` | **URL-format** DSN for golang-migrate: `postgres://user:pass@localhost:5433/toomanyhours?sslmode=disable`. Separate from the app's key=value DSN, because migrate cannot parse that form. |

## Schema

Schema is owned by `golang-migrate`. GORM is the query layer only — `AutoMigrate` is never called, so struct tags do not create constraints.

```bash
migrate -path migrations -database "$MIGRATE_DATABASE_URL" up      # apply
migrate -path migrations -database "$MIGRATE_DATABASE_URL" down 1  # roll back one
migrate create -ext sql -dir migrations -seq <name>                # new migration
```

Postgres data lives in a named Docker volume, so a full reset is `docker compose down -v` — no sudo required.

`sql/seed.sql` holds content, not schema, and is never applied automatically. It creates a local fixture account:

```
email:    admin@example.com
password: devpassword
```

Registration reserves the username `admin`, so this row can only be created by the seed. It is a local development fixture — do not deploy it anywhere reachable from the internet.

## Tests

```bash
go test ./...
```

`internal/validate` is the covered package: username, email and password rules, including bcrypt's 72-**byte** ceiling, past which bcrypt silently truncates the input.
