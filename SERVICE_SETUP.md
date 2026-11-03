# Controlled service setup

This runbook covers the Go API, PostgreSQL, optional providers, tests, and common local-development failures.

## 1. Fastest start: Docker Compose

Requirements: Docker with Compose support.

From the repository root:

```bash
docker compose up --build -d
docker compose ps
curl -i http://127.0.0.1:8080/readyz
```

Readiness is successful when the API returns `204 No Content`. Follow structured logs with:

```bash
docker compose logs -f api
```

The default stack uses:

- PostgreSQL 17;
- the fake, process-local custody provider;
- deterministic guidance (`CONTROLLED_AI_PROVIDER=disabled`);
- no Channel3 price checks unless a key is supplied; and
- Expo push delivery without enhanced-push authentication.

Stop while preserving database data:

```bash
docker compose down
```

Delete the local database volume only when you intentionally want a clean reset:

```bash
docker compose down --volumes
```

That command permanently removes Compose-managed Controlled data.

## 2. Configuration

Copy the example when enabling external providers:

```bash
cp .env.example .env
```

Docker Compose reads `.env` automatically. A direct `go run` does not; export or source variables into that shell first.

### Core

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DATABASE_URL` | direct run | none | PostgreSQL URL; Compose supplies one |
| `CONTROLLED_HTTP_ADDR` | no | `:8080` | API listen address |
| `CONTROLLED_PORT` | Compose only | `8080` | host port mapped to the API |
| `CONTROLLED_LOG_LEVEL` | no | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `CONTROLLED_SHUTDOWN_TIMEOUT` | no | `10s` | graceful shutdown timeout |
| `CONTROLLED_DB_PING_TIMEOUT` | no | `5s` | startup/readiness database timeout |
| `CONTROLLED_DB_MAX_CONNS` | no | `10` | maximum pool size |
| `CONTROLLED_DB_MIN_CONNS` | no | `1` | minimum pool size |
| `CONTROLLED_GOOGLE_CLIENT_ID` | no | project Web client ID | accepted Google ID-token audience |

### Custody

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `CONTROLLED_CUSTODY_PROVIDER` | no | `fake` | `fake` or `paypal_sandbox` |
| `PAYPAL_API_BASE` | with PayPal | Sandbox API | PayPal API origin |
| `PAYPAL_CLIENT_ID` | with PayPal | none | Sandbox REST app client ID |
| `PAYPAL_CLIENT_SECRET` | with PayPal | none | Sandbox REST app secret |
| `PAYPAL_RETURN_URL` | with PayPal | example URL | order approval return URL |
| `PAYPAL_CANCEL_URL` | with PayPal | example URL | order approval cancellation URL |

### Guidance, shopping, and push

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `CONTROLLED_AI_PROVIDER` | no | `disabled` | `disabled` or `openai_compatible` |
| `CONTROLLED_AI_BASE_URL` | with AI | OpenRouter API | compatible `/chat/completions` base |
| `CONTROLLED_AI_MODEL` | with AI | none | structured-output-capable model ID |
| `CONTROLLED_AI_API_KEY` | with AI | none | server-side AI credential |
| `CONTROLLED_AI_TIMEOUT` | no | `8s` | must be below API write timeout |
| `CONTROLLED_AI_REQUIRE_PARAMETERS` | no | `true` | require provider structured-output support |
| `CHANNEL3_API_BASE` | no | Channel3 v1 | product catalog API base |
| `CHANNEL3_API_KEY` | for price checks | none | server-side Channel3 key |
| `CHANNEL3_TIMEOUT` | no | `4s` | catalog request timeout |
| `CONTROLLED_EXPO_PUSH_URL` | no | Expo Push API | delivery endpoint |
| `CONTROLLED_EXPO_ACCESS_TOKEN` | enhanced push only | none | Expo enhanced-security token |
| `CONTROLLED_EXPO_PUSH_TIMEOUT` | no | `5s` | push request timeout |

Durations use Go syntax such as `500ms`, `5s`, and `1m`. Do not put provider secrets in the Expo app or any `EXPO_PUBLIC_*` variable.

Startup logs report each feature/provider as loaded or disabled without printing secrets.

## 3. PayPal Sandbox mode

Set these values in `.env`:

```dotenv
CONTROLLED_CUSTODY_PROVIDER=paypal_sandbox
PAYPAL_CLIENT_ID=your-sandbox-client-id
PAYPAL_CLIENT_SECRET=your-sandbox-client-secret
PAYPAL_RETURN_URL=https://your-return-page.example/paypal/return
PAYPAL_CANCEL_URL=https://your-return-page.example/paypal/cancel
```

Then rebuild/restart the API:

```bash
docker compose up --build -d api
docker compose logs -f api
```

The funding endpoint returns a PayPal approval URL. After buyer approval, the mobile app calls the capture endpoint. Withdrawals use PayPal Payouts and can be reconciled through the API.

The repository also includes standalone credential/provider harnesses:

```bash
export PAYPAL_CLIENT_ID='your-sandbox-client-id'
export PAYPAL_CLIENT_SECRET='your-sandbox-client-secret'

scripts/paypal_sandbox_funding.sh run 10.00 USD
scripts/paypal_sandbox_payout.sh send RECIPIENT_SANDBOX_EMAIL 1.00 USD
```

These scripts exercise PayPal directly; they do not post to the Controlled ledger.

## 4. Direct local run

Requirements:

- Go `1.27.2` or compatible newer toolchain
- PostgreSQL 17
- `curl`

Create a local database and export its URL, for example:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_dev?sslmode=disable'
go run ./cmd/api
```

The API applies migrations at startup. There is no separate migration command. Keep the process running and verify it in another terminal:

```bash
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/readyz
```

Both should return `204`. Liveness checks only the process; readiness also pings PostgreSQL.

## 5. API container by itself

Build the image:

```bash
docker build -t controlled-api:local .
```

Run it against a reachable PostgreSQL database:

```bash
docker run --rm \
  --name controlled-api \
  -p 8080:8080 \
  -e CONTROLLED_HTTP_ADDR=':8080' \
  -e DATABASE_URL='postgres://USER:PASSWORD@DATABASE_HOST:5432/controlled?sslmode=require' \
  controlled-api:local
```

Inside a container, `localhost` refers to that container; use the database host/service name. The image runs as a non-root user and contains only the API binary and runtime certificates.

## 6. Tests

Run the standard backend suite:

```bash
go test ./...
```

PostgreSQL integration tests run only when a disposable test database URL is supplied:

```bash
export CONTROLLED_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/controlled_test?sslmode=disable'
go test -v ./...
go test -race ./...
```

Never point `CONTROLLED_TEST_DATABASE_URL` at data you need to preserve. Integration tests create and remove fixtures.

Mobile checks:

```bash
cd app
npm install
npm run typecheck
npm run lint
npx expo-doctor
```

## 7. Database and migrations

Inspect Compose PostgreSQL without exposing its port:

```bash
docker compose exec postgres \
  psql -U controlled -d controlled \
  -c 'SELECT version, name, applied_at FROM schema_migrations ORDER BY version'
```

There are currently 18 migrations, ending with persisted accountability denial reasons. Applied migration files must not be edited: the runner stores checksums and refuses to start if history changes. Add a new up/down migration for schema evolution.

## 8. Fake-provider boundary

The fake custody provider keeps pending provider orders/payouts in the API process. PostgreSQL wallet data survives restart, but an unfinished fake provider interaction may need a new intent after restart. Complete local fake funding/settlement before restarting when possible.

The PayPal Sandbox provider is external and does not share this process-local limitation, although production-grade webhook processing is still outside MVP scope.

## 9. Logs

Logs are structured JSON. Useful startup messages include:

- `Database connection established`
- `Database migrations completed`
- `Backend feature status`
- `Backend features initialized`
- `Controlled API listening`

At `INFO`, requests include method, path, status, and duration. Domain/provider failures include an error and safe identifiers. Tokens, passwords, and provider secrets are not logged.

## 10. Troubleshooting

### `DATABASE_URL is required`

Compose supplies it. For `go run`, export it in the same shell.

### Readiness returns `503`

Check PostgreSQL and API logs:

```bash
docker compose ps
docker compose logs postgres api
```

For a direct run:

```bash
psql "$DATABASE_URL" -c 'SELECT 1'
```

### PayPal provider rejects startup

When `CONTROLLED_CUSTODY_PROVIDER=paypal_sandbox`, client ID, secret, API base, return URL, and cancel URL must all be non-empty.

### AI is logged as disabled

Set `CONTROLLED_AI_PROVIDER=openai_compatible` plus base URL, model, and API key. If disabled, guidance still works through deterministic rules.

### Shopping is logged as disabled

Set `CHANNEL3_API_KEY` in the API environment and restart. Price checks are optional; transfers remain available without them.

### Physical phone cannot reach the API

Bind the API on `:8080`, put the phone and development machine on the same network, allow the port through the host firewall, and launch Metro with:

```bash
cd app
EXPO_PUBLIC_API_URL=http://YOUR-LAN-IP:8080 npm start
```

Do not use `localhost` or `10.0.2.2` from a physical phone.

### Push notifications do not arrive

Use a physical device, grant notification permission, ensure the EAS project/FCM setup described in [app/README.md](./app/README.md) is present, and inspect API logs for token registration or Expo delivery errors.
