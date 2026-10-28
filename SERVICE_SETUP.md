# Controlled service setup

This runbook starts the complete backend currently implemented in this repository:

- PostgreSQL 17 for users, sessions, wallets, operations, and the ledger
- the Controlled Go HTTP API

The API applies embedded database migrations at startup. There is no separate migration command or migration service.

## Fastest start: Docker Compose

The repository includes a multi-stage `Dockerfile` and a `compose.yaml` stack. From the repository root, build and start PostgreSQL 17 plus the API:

```bash
docker compose up --build -d
```

Watch startup and migration logs:

```bash
docker compose logs -f api
```

Check service state and readiness:

```bash
docker compose ps
curl -i http://127.0.0.1:8080/readyz
```

The API is ready when Compose reports both services as healthy and readiness returns `204 No Content`. Continue with [API_INTEGRATION_SPEC.md](./API_INTEGRATION_SPEC.md) to exercise the API.

Stop the stack while preserving PostgreSQL data:

```bash
docker compose down
```

Start it again without rebuilding:

```bash
docker compose up -d
```

To deliberately remove the Compose-managed database as well, run `docker compose down --volumes`. This permanently deletes local Controlled database data.

### Compose configuration

Defaults are intended only for local development. Override them through shell variables or a local `.env` file:

| Variable | Default | Purpose |
| --- | --- | --- |
| `CONTROLLED_PORT` | `8080` | host port mapped to the API |
| `POSTGRES_DB` | `controlled` | database created by the PostgreSQL container |
| `POSTGRES_USER` | `controlled` | local PostgreSQL user |
| `POSTGRES_PASSWORD` | `controlled-local-only` | local PostgreSQL password |
| `DATABASE_URL` | Compose PostgreSQL URL | complete API database connection URL |
| `CONTROLLED_LOG_LEVEL` | `INFO` | API log level |
| `CONTROLLED_SHUTDOWN_TIMEOUT` | `10s` | graceful shutdown timeout |
| `CONTROLLED_DB_PING_TIMEOUT` | `5s` | database ping timeout |
| `CONTROLLED_DB_MAX_CONNS` | `10` | maximum connection-pool size |
| `CONTROLLED_DB_MIN_CONNS` | `1` | minimum connection-pool size |

If you change `POSTGRES_DB`, `POSTGRES_USER`, or `POSTGRES_PASSWORD`, also set a matching `DATABASE_URL`; its default is a literal URL using the default credentials.

Example local overrides:

```bash
CONTROLLED_PORT=18080 \
DATABASE_URL='postgres://controlled:controlled-local-only@postgres:5432/controlled?sslmode=disable' \
docker compose up --build -d
```

PostgreSQL is intentionally not published to a host port in the Compose stack. Inspect it through the container:

```bash
docker compose exec postgres \
  psql -U controlled -d controlled \
  -c 'SELECT version, name, applied_at FROM schema_migrations ORDER BY version'
```

## Build and run the API image directly

Build a deployable Linux image:

```bash
docker build -t controlled-api:local .
```

The image contains a statically built Go binary, CA certificates, a non-root runtime user, and an HTTP readiness health check. It does not contain PostgreSQL.

Run it against an existing database reachable from the container:

```bash
docker run --rm \
  --name controlled-api \
  -p 8080:8080 \
  -e CONTROLLED_HTTP_ADDR=':8080' \
  -e DATABASE_URL='postgres://USER:PASSWORD@DATABASE_HOST:5432/controlled?sslmode=require' \
  controlled-api:local
```

Replace the database URL with credentials and TLS settings appropriate for the deployment. Do not use `localhost` for a database in another container: inside the API container, `localhost` refers to the API container itself.

For a remote deployment, terminate HTTPS at the platform load balancer or reverse proxy, inject `DATABASE_URL` from the platform's secret manager, keep PostgreSQL off the public network, and persist the database independently of the API container. The API image is stateless except for the current process-local fake custody provider described below.

## Current development boundary

This is the hackathon backend, not a production deployment:

- custody and payout behavior is provided by an in-memory fake provider;
- funding confirmation and withdrawal settlement use `/v1/demo/*` endpoints;
- the debit policy currently allows valid wallet transfers and withdrawals; pact enforcement is not connected yet;
- the API is HTTP-only when run locally; and
- there is no frontend service in this repository yet.

Because fake custody state is held in the API process, finish any pending funding or withdrawal simulation before restarting the API. PostgreSQL data survives a restart, but the fake provider's pending orders do not.

## Prerequisites

Install:

- Go `1.27.2` or a compatible newer Go toolchain
- Docker
- `curl`
- `jq` for the API walkthrough
- PostgreSQL 17 client tools, including `psql-17`, for direct database checks

Verify the main tools:

```bash
go version
docker version
psql-17 --version
curl --version
jq --version
```

Run all commands below from the repository root:

```bash
cd /Users/raagan/controlled
```

## 1. Start PostgreSQL 17

For a new local container:

```bash
docker run --name controlled-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  -v controlled-postgres-data:/var/lib/postgresql/data \
  -d postgres:17
```

If that container already exists but is stopped:

```bash
docker start controlled-postgres
```

Wait until PostgreSQL is ready:

```bash
docker exec controlled-postgres pg_isready -U postgres
```

Expected output ends with `accepting connections`.

If another PostgreSQL container is already listening on port `5432`, use it instead and skip the container creation command.

## 2. Create the development database

Check whether the database exists:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -tAc "SELECT 1 FROM pg_database WHERE datname = 'controlled_dev'"
```

If the command prints nothing, create it:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -c 'CREATE DATABASE controlled_dev'
```

Confirm connectivity, then exit automatically:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/controlled_dev?sslmode=disable' \
  -c 'SELECT current_database(), current_user, version()'
```

## 3. Download Go dependencies

```bash
go mod download
```

## 4. Start the API

In a terminal from the repository root:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_dev?sslmode=disable'
export CONTROLLED_HTTP_ADDR='127.0.0.1:8080'
export CONTROLLED_LOG_LEVEL='INFO'
go run ./cmd/api
```

A successful start logs a JSON message containing:

```text
Controlled API listening
```

The startup sequence connects to PostgreSQL, checks all previously applied migration checksums, applies pending migrations under an advisory lock, and then starts HTTP traffic.

Keep this terminal open. Press `Ctrl-C` for a graceful API shutdown.

## 5. Verify health

In a second terminal:

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/readyz
```

Both endpoints should return `HTTP/1.1 204 No Content`. Liveness only confirms that the process is running. Readiness also confirms that the API can reach PostgreSQL.

The backend is now ready. Continue with [API_INTEGRATION_SPEC.md](./API_INTEGRATION_SPEC.md) for a complete external API flow.

## Configuration reference

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | none | PostgreSQL connection URL |
| `CONTROLLED_HTTP_ADDR` | no | `:8080` | HTTP listen address |
| `CONTROLLED_LOG_LEVEL` | no | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `CONTROLLED_SHUTDOWN_TIMEOUT` | no | `10s` | graceful HTTP shutdown timeout |
| `CONTROLLED_DB_PING_TIMEOUT` | no | `5s` | database connection/readiness timeout |
| `CONTROLLED_DB_MAX_CONNS` | no | `10` | maximum PostgreSQL pool connections |
| `CONTROLLED_DB_MIN_CONNS` | no | `1` | minimum PostgreSQL pool connections |

Durations use Go duration syntax, such as `500ms`, `5s`, or `1m`. The minimum connection count cannot exceed the maximum.

## Run the test suite

Unit tests do not require a database:

```bash
go test ./...
```

For PostgreSQL integration tests, create a separate database once:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -tAc "SELECT 1 FROM pg_database WHERE datname = 'controlled_test'"
```

If that check prints nothing:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -c 'CREATE DATABASE controlled_test'
```

Run all tests, including integration tests:

```bash
CONTROLLED_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_test?sslmode=disable' \
  go test -v ./...
```

Run the race detector against the same suite:

```bash
CONTROLLED_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_test?sslmode=disable' \
  go test -race ./...
```

The integration tests manage their own fixtures but expect a reachable, disposable test database. Never point `CONTROLLED_TEST_DATABASE_URL` at a database containing data you need to preserve.

## Inspect migrations

With the API started at least once:

```bash
psql-17 'postgres://postgres:postgres@localhost:5432/controlled_dev?sslmode=disable' \
  -c 'SELECT version, name, applied_at FROM schema_migrations ORDER BY version'
```

The expected versions are:

1. `000001_wallet_ledger.up.sql`
2. `000002_sessions.up.sql`
3. `000003_wallet_operations.up.sql`

Applied migration files must not be edited. The API stores their checksums and refuses to start if an applied migration changes.

## Stop and restart

Stop the API with `Ctrl-C` in its terminal. Stop PostgreSQL without deleting its data:

```bash
docker stop controlled-postgres
```

Restart later with:

```bash
docker start controlled-postgres
```

Then run the API command from step 4 again. The named Docker volume preserves the database.

## Troubleshooting

### Port 5432 is already allocated

Check running containers:

```bash
docker ps
```

Either use the PostgreSQL instance already bound to `5432`, or map the Controlled container to another host port such as `5433` and update both database URLs.

### `DATABASE_URL is required`

Export `DATABASE_URL` in the same terminal that runs `go run ./cmd/api`.

### Readiness returns 503

Check PostgreSQL first:

```bash
psql-17 "$DATABASE_URL" -c 'SELECT 1'
```

Then inspect the API log for authentication, connection, or migration errors.

### A demo funding confirmation or payout settlement fails after restart

The fake custody provider is process-local. Create a new funding intent or withdrawal and complete it without restarting the API. Production custody recovery and webhook reconciliation are not implemented yet.
