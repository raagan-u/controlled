# Controlled

**A PayPal-powered, intent-aware wallet that helps people keep the financial promises they make to themselves.**

Controlled is part of the *From My Heart* series and is being built for the PayPal AI Hackathon.

## Why Controlled?

Most budgeting apps explain overspending after the money is gone. Controlled intervenes before the debit happens.

Users can send, receive, and spend normally. When they want more control, they create an account-wide pact with a goal, a spending limit, a duration, and an additional financial stake. While a pact is active, every outgoing transaction is checked against it.

## How it works

1. Fund the Controlled wallet through PayPal.
2. State a financial goal and create a daily or weekly spending pact.
3. Back the budget with an additional stake held separately from spendable funds.
4. Add n ( n>=1) trusted accountability partners.
5. Spend normally while remaining within every active pact.
6. If a transaction exceeds a limit, request approval from the next partner in round-robin order.

A denied request leaves the transaction blocked. An approved emergency allows the expense and preserves the active stake. An approved conscious break allows the expense but moves the stake into the user's **Commitment Reserve**.

## MVP break consequence

Before activating a pact, the user must explicitly select what happens to the stake after a conscious break. The initial MVP enables one option:

- **Commitment Reserve:** the stake remains the user's money but becomes unavailable to spend, send, or withdraw. It can be unlocked by successfully completing a recovery pact.

The onboarding interface will also show options such as a neutral fund, goal vault, and split consequence as disabled **Coming soon** choices. Controlled does not receive forfeited or reserved stakes.

PYUSD is not part of the initial MVP. We intend to explore PYUSD rewards or other responsible ways for locked stakes to create value instead of remaining idle, but no reward, yield, or investment return is currently promised.

## AI in Controlled

AI understands the intent behind a pact, but it never moves money or replaces human accountability.

- **AI Pact Planner:** analyzes wallet history and the user's goal to suggest a realistic limit, duration, and stake.
- **AI Accountability Brief:** summarizes the transaction, affected pacts, verified wallet facts, and the user's explanation for the assigned partner.

Hard balance and budget rules remain deterministic. Humans approve exceptions.

> AI advises. Humans authorize. The policy engine enforces. PayPal moves the money.

## Hackathon build

The MVP demonstrates:

- A wallet ledger funded through PayPal Sandbox
- Free sending and receiving when no pact is active
- Account-wide enforcement of one or more active pacts
- Stake locking, release, and Commitment Reserve recovery flows
- n-partner round-robin accountability (n >= 1)
- Emergency and conscious-break outcomes
- AI-assisted planning and factual approval briefs
- An auditable timeline of financial and accountability decisions

The prototype is a sandbox proof of concept, not a production bank, custodial wallet, investment product, or rewards service.

## Business model

Controlled may charge a small, transparent fee when a pact activates. The fee is independent of whether the user succeeds or breaks the pact and remains separate from stakes, Commitment Reserves, provider fees, and any future user rewards.

## Technology

- Go modular-monolith backend
- Expo React Native mobile app for Android and iOS
- PostgreSQL ledger and audit history
- PayPal Sandbox payment flows
- AI with structured, grounded outputs

See the [product specification](./SPEC.md) and [backend specification](./BACKEND_SPEC.md) for the evolving system design. To run the mobile UI, use the [app setup guide](./app/README.md). To run and exercise the implemented backend, use the [service setup runbook](./SERVICE_SETUP.md) and [external API integration specification](./API_INTEGRATION_SPEC.md).
