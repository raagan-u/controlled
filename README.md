# Controlled

**A wallet that helps people keep the financial promises they make to themselves.**

Controlled is a polished Android hackathon MVP built for the PayPal AI Hackathon. It combines a USD wallet, enforceable spending pacts, human accountability, optional AI guidance, purchase price checks, and mobile notifications.

## Live MVP

- **Product:** [tryctrld.xyz](https://tryctrld.xyz)
- **Android APK:** [download the latest release](https://github.com/raagan-u/controlled/releases/latest/download/ctrld.apk)
- **API readiness:** [api.tryctrld.xyz/readyz](https://api.tryctrld.xyz/readyz)
- **API container:** [`ghcr.io/raagan-u/controlled-api:stable`](https://github.com/raagan-u/controlled/pkgs/container/controlled-api)

The hosted product and APK use PayPal Sandbox and are intended for demonstration and testing only.

> [!NOTE]
> **Top post-MVP priority — earned pact-skip passes:** a scarce, auditable fallback for an unresponsive accountability circle. One pass would authorize one exact, otherwise-valid transaction; successful pact streaks would earn replacements. This is not implemented in the submission build.

## The idea

Most budgeting tools report overspending after it happens. Controlled checks intent before money leaves.

A user creates a pact with a goal, spending limit, time period, financial stake, and an ordered circle of trusted partners. Every outgoing transfer, payment-request payment, and withdrawal is evaluated against all active pacts. Ordinary spending completes immediately. Spending that would exceed a pact is paused and sent to an accountability partner.

The reviewer can:

- approve an **emergency**, allowing this exact transaction while keeping the pact and stake active;
- approve a **conscious break**, allowing the transaction, resolving the affected pact as broken, and moving its stake to the user's Commitment Reserve; or
- deny the request with a required reason, after which the request advances to the next eligible partner or ends when the circle is exhausted.

Insufficient balance is never overridable. AI never moves money or approves an exception.

## What is implemented

### Mobile product

- Email/password registration and sign-in
- Native Google sign-in
- First-time display-name setup
- Home dashboard with available balance, recent recipients, live pact progress, and transaction activity
- PayPal Sandbox or fake-provider wallet funding
- Internal transfers by Controlled recipient, with required meaningful persisted remarks
- Payment requests with pay, decline, cancel, expiry, and pact enforcement
- Withdrawals with required reasons and provider settlement/reconciliation
- Optional purchase-intent price checks through Channel3
- Pact creation with a 24-hour minimum, local draft recovery, scheduling, progress, history, and outcomes
- Stakes & recovery view with live totals, seven-day eligibility, projected unlocks, and clear ineligible states
- Accountability invitations, ordered circles, approval classifications, denial reasons, and decision history
- Transaction details showing remarks and pact-approval metadata
- Expo push notifications for money received, payment requests, invitations, and accountability decisions
- Automatic Home refresh when returning, foregrounding, or receiving server-side changes during an active session

### Backend

- Go modular monolith with strict JSON HTTP APIs
- PostgreSQL 17 persistence and embedded, checksum-verified migrations
- Double-entry USD ledger for available, pending, staked, and Commitment Reserve balances
- Atomic policy checks across overlapping active pacts
- Idempotent money-moving and resource-creation commands
- Ordered accountability assignment with six-hour reviewer windows and 24-hour request expiry
- Pact and request maintenance on startup and once per minute
- Atomic oldest-first Commitment Reserve recovery from kept pacts lasting at least seven days
- Selectable fake or PayPal Sandbox custody provider
- Active-pact transaction previews grounded in 30-day spending patterns, with deterministic guidance and optional OpenAI-compatible structured-output enhancement
- Optional Channel3 product search
- Asynchronous Expo push delivery
- Structured Zap startup, feature-status, request, domain, and error logs

## Core flow

1. Sign up with email/password or Google.
2. Fund the Controlled wallet.
3. Invite at least one accountability partner and create an ordered circle.
4. Create a pact with a goal, limit, stake, and a period of at least 24 hours. Seven days or longer qualifies it for reserve recovery.
5. Confirm the terms and reserve the stake.
6. Send with a meaningful remark, withdraw with a meaningful reason, or pay an existing request. When an active pact exists, Controlled previews the payment against the pact and recent spending pattern; without an active pact, guidance is skipped.
7. If a debit crosses any active pact limit, explain why and request approval.
8. The assigned partner approves it as an emergency/conscious break or denies it with a reason.
9. Controlled rechecks balance and every current pact before executing an approved transaction.
10. If a pact breaks, its stake remains user-owned in Commitment Reserve; later kept pacts lasting at least seven days unlock it up to their own stake, oldest reserve first.

## System shape

```text
Expo React Native app
        |
        | Bearer session + JSON
        v
Go HTTP API
  |-- Identity and sessions
  |-- Wallet operations and payment requests
  |-- Pact policy and accountability
  |-- AI guidance / Channel3 / Expo push
  |
  +--> PostgreSQL double-entry ledger
  +--> fake custody or PayPal Sandbox
```

## Quick start

Start PostgreSQL and the API from the repository root:

```bash
docker compose up --build -d
curl -i http://127.0.0.1:8080/readyz
```

The Compose defaults use the process-local fake custody provider and deterministic guidance, so no provider secrets are needed for the basic flow.

Start the mobile app in another terminal:

```bash
cd app
npm install
npm run android
```

Google sign-in and notifications use native modules, so use a development build rather than Expo Go. An Android emulator reaches the API at `http://10.0.2.2:8080`; iOS uses `http://127.0.0.1:8080`. For a physical device:

```bash
EXPO_PUBLIC_API_URL=http://YOUR-LAN-IP:8080 npm start
```

## Verification

```bash
go test ./...

cd app
npm run typecheck
npm run lint
```

PostgreSQL integration tests additionally use `CONTROLLED_TEST_DATABASE_URL`; see the service runbook.

## Optional providers

Copy [`.env.example`](./.env.example) to `.env` to configure:

- `CONTROLLED_CUSTODY_PROVIDER=paypal_sandbox` plus PayPal Sandbox credentials;
- `CONTROLLED_AI_PROVIDER=openai_compatible` plus a structured-output-capable model;
- `CHANNEL3_API_KEY` for purchase price checks; and
- `CONTROLLED_EXPO_ACCESS_TOKEN` only when Expo enhanced push security is enabled.

Secrets belong only in the backend environment. Never place PayPal, AI, Channel3, or Expo access tokens in `EXPO_PUBLIC_*` variables.

## Documentation

- [Product specification](./SPEC.md) — implemented behavior and UX
- [Backend specification](./BACKEND_SPEC.md) — architecture, invariants, persistence, and providers
- [Service setup](./SERVICE_SETUP.md) — local, Docker, provider, test, and troubleshooting instructions
- [HTTP API reference](./API_INTEGRATION_SPEC.md) — protocol and endpoint contracts
- [Mobile app guide](./app/README.md) — native configuration and development workflow

## Future scope

These are post-MVP ideas and would require appropriate security, provider, legal, and compliance review:

1. **Earned pact-skip passes:** a limited, audited fallback when an accountability circle is unresponsive, replenished through successful pact streaks.
2. **Control profile and score:** a transparent view of pact consistency, protected money, exceptions, passes, and long-term progress—not a credit score.
3. **Verified account recovery:** email verification, password reset, session revocation, and recovery alerts.
4. **iOS and Sign in with Apple:** production-ready iOS support with Apple/iCloud authentication.
5. **QR payments:** scan or share recipients, merchants, payment requests, and prefilled payments without bypassing confirmation or pact checks.
6. **Merchant recognition:** identify merchants from verified recipient/payment metadata and ask when classification is ambiguous.
7. **Transfer categories and richer planning:** editable categorization that powers category-aware pacts and a more detailed dashboard.
8. **Stronger AI guidance:** better models and merchant context, while financial policy and authorization remain deterministic and human-controlled.
9. **External transfers:** send to verified PayPal accounts outside Controlled with safe failure and reconciliation flows.
10. **PYUSD settlement:** explicit USD/PYUSD conversion, movement, fees, balances, and reconciliation.
11. **On-chain commitments:** audited smart contracts and Web3 wallets for verifiable pact custody and release.
12. **Optional stake yield:** opt-in, risk-disclosed low-risk yield treatment with principal protection rules and no guaranteed returns.

## MVP boundaries

Controlled is a sandbox proof of concept, not a bank, production custodial wallet, money-transmission service, investment product, or source of yield. It is USD-only. Password reset and email verification are not implemented. Commitment Reserve stays user-owned, cannot be withdrawn directly, and is recovered only through later kept pacts lasting at least seven days. The fake provider is process-local; PayPal integration targets Sandbox only.

> AI advises. Humans authorize. The policy engine enforces. The provider moves the money.
