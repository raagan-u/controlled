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
| `CONTROLLED_GOOGLE_CLIENT_ID` | Controlled Web OAuth client ID | accepted audience for Google ID tokens |
| `CONTROLLED_AI_PROVIDER` | `disabled` | `disabled` for deterministic guidance, or `openai_compatible` |
| `CONTROLLED_AI_BASE_URL` | `https://openrouter.ai/api/v1` | OpenAI-compatible provider base URL |
| `CONTROLLED_AI_MODEL` | empty | provider model identifier; required when AI is enabled |
| `CONTROLLED_AI_API_KEY` | empty | provider secret; required when AI is enabled |
| `CONTROLLED_AI_TIMEOUT` | `8s` | per-provider request timeout, below the API write timeout |
| `CONTROLLED_AI_REQUIRE_PARAMETERS` | `true` | require an OpenRouter route that supports structured output parameters |
| `CONTROLLED_EXPO_PUSH_URL` | Expo Push API | push delivery endpoint |
| `CONTROLLED_EXPO_ACCESS_TOKEN` | empty | optional Expo enhanced-push-security token |
| `CONTROLLED_EXPO_PUSH_TIMEOUT` | `5s` | per-push request timeout |

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

Google sign-in uses `POST /v1/auth/google`. The app sends a Google ID token; the API verifies its signature, issuer, expiry, verified email, and Web client-ID audience before issuing a Controlled session. OAuth client IDs are public configuration values—never add a Google client secret to the mobile app.

AI guidance is optional and read-only. With `CONTROLLED_AI_PROVIDER=disabled`, the same endpoints return deterministic rule-based guidance so pact and accountability flows remain available. To enable an OpenAI-compatible provider such as OpenRouter, set the provider, model, and API key in the API process environment. The key must never be placed in the Expo app or an `EXPO_PUBLIC_*` variable.

The backend sends only a curated financial snapshot, requires strict structured JSON, validates the response, and falls back to deterministic guidance on timeouts, rate limits, or invalid output. The model has no wallet-command, ledger, approval, or pact-lifecycle capability.

Email/password registration and login use `POST /v1/auth/password/register` and `POST /v1/auth/password/login`. Passwords are bcrypt-hashed by the API and are never returned or stored by the mobile client. Email OTP verification is intentionally deferred during the current onboarding pass.

## System notifications

Controlled uses Expo Push Service for Android system notifications. The API stores Expo push tokens by authenticated user and sends only actionable events: payment requests, accountability decisions, and accountability invitations. Tapping one opens the existing request, accountability, or invitation screen; there is intentionally no separate notification inbox.

`CONTROLLED_EXPO_ACCESS_TOKEN` is only needed if enhanced push security is enabled for the Expo project. Keep that token on the API server. Notification delivery is best-effort and never blocks a wallet or approval operation; pending-action badges remain the source of truth.

## Current development boundary

This is the hackathon backend, not a production deployment:

- custody and payout behavior is provided by an in-memory fake provider;
- funding confirmation and withdrawal settlement use `/v1/demo/*` endpoints;
- outgoing transfers and withdrawals are atomically enforced against every active pact;
- accountability assignment and pact lifecycle maintenance runs at startup and once per minute;
- the API is HTTP-only when run locally; and
- there is no frontend service in this repository yet.

Because fake custody state is held in the API process, finish any pending funding or withdrawal simulation before restarting the API. PostgreSQL data survives a restart, but the fake provider's pending orders do not.

## Test PayPal Sandbox funding directly

The standalone funding harness exercises the real PayPal Sandbox Orders v2 flow without changing the API's current fake-custody implementation. It creates an order for the Controlled Sandbox Business account, asks a Sandbox Personal buyer to approve it, and captures the approved payment.

Install `curl` and `jq`, then export the REST app credentials belonging to the Controlled Sandbox Business account:

```bash
export PAYPAL_CLIENT_ID='your-sandbox-client-id'
export PAYPAL_CLIENT_SECRET='your-sandbox-client-secret'
```

Run an interactive USD 10 funding test:

```bash
scripts/paypal_sandbox_funding.sh run 10.00 USD
```

The script prints the PayPal approval URL. Open it, sign in with the Sandbox Personal test account, approve the payment, and return to the terminal to capture it. The configured return URL does not need to be running for this terminal-driven test.

To create and capture in separate terminal commands:

```bash
scripts/paypal_sandbox_funding.sh create 10.00 USD
scripts/paypal_sandbox_funding.sh capture PAYPAL_ORDER_ID
```

The capture output includes the PayPal capture ID, status, gross amount, PayPal fee, and net amount. Keep the capture ID for subsequent refund testing. Do not commit Sandbox credentials or use live credentials with this script.

### Test a withdrawal with PayPal Payouts

To send a separate wallet withdrawal from the Controlled Sandbox Business account to a Sandbox Personal PayPal account, use the recipient's Sandbox email address:

```bash
scripts/paypal_sandbox_payout.sh send RECIPIENT_SANDBOX_EMAIL 1.00 USD
```

The script prints the payout batch ID and retrieves its initial item status. Payout processing can be asynchronous; retrieve its latest state with:

```bash
scripts/paypal_sandbox_payout.sh status PAYOUT_BATCH_ID
```

Verify the withdrawal in the Business account's Sandbox activity and the receipt in the Personal account's Sandbox activity. A payout is a new transfer to a PayPal recipient and is not a refund of the original checkout funding source. To return an original payment to its original source instead, use the Payments v2 capture-refund endpoint.

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
| `CONTROLLED_AI_PROVIDER` | no | `disabled` | `disabled` or `openai_compatible` |
| `CONTROLLED_AI_BASE_URL` | with AI | OpenRouter API | provider base URL |
| `CONTROLLED_AI_MODEL` | with AI | none | structured-output-capable model ID |
| `CONTROLLED_AI_API_KEY` | with AI | none | server-side provider credential |
| `CONTROLLED_AI_TIMEOUT` | no | `8s` | provider request timeout |
| `CONTROLLED_AI_REQUIRE_PARAMETERS` | no | `true` | require structured-output parameter support |

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

Run only the pact stake lifecycle integration test:

```bash
CONTROLLED_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_test?sslmode=disable' \
  go test -v ./internal/pact -run TestPostgresPactStakeLifecycle
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
4. `000004_google_identities.up.sql`
5. `000005_password_credentials.up.sql`
6. `000006_accountability_relationships.up.sql`
7. `000007_pacts.up.sql`
8. `000008_pact_stakes.up.sql`
9. `000009_pact_usage.up.sql`
10. `000010_accountability_requests.up.sql`

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
