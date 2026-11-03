# Controlled

**A wallet that helps people keep the financial promises they make to themselves.**

Controlled is a polished hackathon MVP built for the PayPal AI Hackathon. It combines a USD wallet, enforceable spending pacts, human accountability, optional AI guidance, purchase price checks, and mobile notifications in one Android/iOS experience.

> [!NOTE]
> **Top post-MVP priority — earned pact-skip passes:** when an accountability circle remains unresponsive beyond a defined waiting period, a user could spend one earned pass to authorize one exact, otherwise-valid transaction. Passes would be strictly limited, fully audited, unable to override insufficient balance, and replenished one at a time after a configured number of successfully completed pacts. This resilience mechanism is part of the roadmap and is not implemented in the current submission build.

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
- Internal transfers by Controlled recipient, with persisted remarks
- Payment requests with pay, decline, cancel, expiry, and pact enforcement
- Withdrawals with provider settlement/reconciliation
- Optional purchase-intent price checks through Channel3
- Pact creation, local draft recovery, scheduling, progress, history, and outcomes
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
- Selectable fake or PayPal Sandbox custody provider
- Deterministic guidance with optional OpenAI-compatible structured-output enhancement
- Optional Channel3 product search
- Asynchronous Expo push delivery
- Structured Zap startup, feature-status, request, domain, and error logs

## Core flow

1. Sign up with email/password or Google.
2. Fund the Controlled wallet.
3. Invite at least one accountability partner and create an ordered circle.
4. Create a pact with a goal, limit, period, and stake.
5. Confirm the terms and reserve the stake.
6. Send, pay, or withdraw normally.
7. If a debit crosses any active pact limit, explain why and request approval.
8. The assigned partner approves it as an emergency/conscious break or denies it with a reason.
9. Controlled rechecks balance and every current pact before executing an approved transaction.

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

These ideas are not part of the current MVP and would require additional product, security, provider, legal, and compliance validation:

1. **Earned pact-skip passes:** introduce a scarce fallback for cases where an accountability circle remains unresponsive beyond a defined waiting period. A pass would authorize one exact otherwise-valid transaction without waiting for a partner decision, be consumed immediately, and work only while the user has passes remaining. The inventory is fixed and cannot be used to override insufficient balance. After every configured number of successfully completed pacts, the user earns one additional pass. Every award and use must be visible, auditable, rate-limited, and reflected in pact history so this remains an earned resilience mechanism rather than an invisible policy bypass.
2. **Control profile and progress score:** add a dedicated profile that brings together the user's Controlled wallet journey—successful pacts, consistency streaks, emergency exceptions, conscious breaks, earned/used passes, money protected, and progress over time. Present a transparent **Control Score** based only on clearly explained in-product behavior, with a breakdown of what changed it and practical ways to improve; it must not be represented as a credit score or used to obscure financial decisions.
3. **Verified account recovery:** add email verification, forgot-password flows, secure reset tokens, session revocation, and recovery notifications.
4. **Complete iOS support:** harden and release the iOS experience, including Sign in with Apple for users with an Apple ID/iCloud account.
5. **QR support:** let users scan or share QR codes to identify a Controlled recipient, merchant, payment request, or prefilled payment while still showing a confirmation screen and applying ordinary balance and pact checks before money moves.
6. **Dynamic merchant identification:** use verified recipient and payment metadata—not just free-form remarks—to determine whether the recipient is a merchant, resolve a recognizable merchant identity, and improve categorization. Ambiguous recipients should remain uncategorized or request confirmation rather than being silently mislabelled.
7. **Transfer categorization and richer insights:** classify transfers into useful, editable spending categories while preserving the original transaction facts. Once reliable categorization exists, it creates the foundation for a more extensive pact planner that can suggest category-aware limits and goals, plus a more detailed dashboard showing spending patterns, category trends, and progress against each commitment.
8. **Stronger AI guidance:** evaluate and adopt more capable models to improve pact suggestions, progress coaching, accountability briefs, and explanations. Better categorization and merchant context can make this guidance more relevant and personalized, while grounded server facts, structured-output validation, deterministic financial policy, and human authorization remain mandatory.
9. **Transfers outside Controlled:** let users send their Controlled balance to an external PayPal account, similar to withdrawal but with a recipient other than themselves. The current MVP avoids this because an email address alone does not prove that a legitimate or matching PayPal Sandbox account exists; recipient verification and safe failure/reconciliation flows must come first.
10. **PYUSD settlement:** support USD-to-PYUSD conversion and PYUSD movement while keeping the ledger, provider reconciliation, fees, and user-facing balances explicit.
11. **On-chain commitments:** add Web3 wallet support and an audited commitment smart contract that can hold and release pact funds according to verifiable rules, without weakening the existing balance, accountability, or approval safeguards.
12. **Optional stake yield:** with explicit user opt-in, place eligible stakes into carefully selected low-risk yield-generating protocols. When a pact succeeds, return an agreed portion of that pact period's interest to the user. When it fails, the principal follows the pact's stake consequence and that period's interest does not roll into or increase the next pact. This would require transparent risk disclosures, custody design, loss handling, and regulatory review; returns must never be presented as guaranteed.

## MVP boundaries

Controlled is a sandbox proof of concept, not a bank, production custodial wallet, money-transmission service, investment product, or source of yield. It is USD-only. Password reset and email verification are not implemented yet. Commitment Reserve funds remain user-owned but are not releasable in this MVP. The fake provider is process-local; PayPal integration targets Sandbox only.

> AI advises. Humans authorize. The policy engine enforces. The provider moves the money.
