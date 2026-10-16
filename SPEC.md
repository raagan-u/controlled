# Pay Pact — Minimal Product Specification

## Product

Pay Pact is an AI-assisted spending commitment app. A user sets a spending limit, funds a financial stake through PayPal, and selects four accountability partners. Exceeding the limit requires approval from the currently assigned partner.

The platform never profits from a forfeited stake.

## MVP goal

Demonstrate one complete sandbox flow:

1. Create a pact.
2. Fund its stake through PayPal.
3. Attempt spending beyond the limit.
4. Request an override.
5. Route the request to an accountability partner.
6. Approve or deny the request.
7. Preserve or forfeit the stake according to the request type.

## Pact

A pact contains:

- Owner
- Four accountability partners
- Spending limit and currency
- Stake amount
- Start and end time
- Forfeiture destination
- Round-robin partner pointer
- Status

Pact states:

```text
DRAFT -> AWAITING_STAKE -> ACTIVE
ACTIVE -> SUCCEEDED
ACTIVE -> ACTIVE_WITH_EXCEPTION
ACTIVE -> BREACHED
```

An active pact's financial terms cannot be edited.

## Override requests

The owner selects one request type:

- `EMERGENCY_EXCEPTION`: approved access preserves the stake.
- `CONSCIOUS_BREAK`: approved access forfeits the stake.

Request states:

```text
REQUESTED -> ASSIGNED -> APPROVED
                      -> DENIED
                      -> AWAITING_USER
                      -> REASSIGNED
                      -> EXPIRED
```

The assigned partner may respond with:

- Approve
- Deny
- Need more information

AI may summarize the request but cannot approve it or classify an emergency as legitimate.

## Partner assignment

Each pact has four partners arranged in a fixed round-robin order.

- One partner is responsible for a request at a time.
- The owner cannot choose the assigned partner.
- If the partner does not acknowledge within the configured timeout, the request moves to the next partner.
- One valid response resolves the request.
- After each request, the pact advances its round-robin pointer.
- Responses from expired assignments cannot resolve the request.

Every assignment has a unique ID, sequence number, assignment time, expiry time, and status.

## Responsiveness score

Scores measure availability, not approval behaviour.

For each assignment:

| Acknowledgement time | Points |
|---|---:|
| Within 5 minutes | 5 |
| Within 30 minutes | 4 |
| Within 2 hours | 3 |
| Before the deadline | 2 |
| After the deadline | 1 |
| No response | 0 |

Display a rolling score from the last ten assignments:

```text
score = earned points / available points * 100
```

Store raw notification, acknowledgement, and response timestamps. Derive the score from those events so the formula can change later.

## Payments

Use the PayPal sandbox for the prototype.

- Fund a stake using PayPal Checkout.
- Confirm payment state using verified webhooks.
- Return the stake after success.
- Send the stake to a sandbox neutral-fund account after an approved conscious break.
- Use idempotency controls so settlement cannot happen twice.

Pact state and payment state are stored separately.

## AI responsibilities

AI may:

- Convert a conversation into proposed pact terms.
- Identify missing or contradictory terms.
- Summarize an override request for a partner.
- Suggest alternatives before an override.
- Trigger deterministic application actions through validated tools.

AI may not:

- Approve an override.
- Decide whether an emergency is legitimate.
- Change an active pact.
- Move money without the required application state and human authorization.

## Core invariants

1. A pact cannot activate before its stake is funded.
2. An owner cannot approve their own request.
3. Only the active assignment can resolve a request.
4. A pact can be settled only once.
5. Payment retries must be idempotent.
6. Emergency exceptions are recorded without being treated as absolute failures.
7. The platform is never the forfeiture recipient.
8. Every consequential action is recorded in an audit log.

## Out of scope

- Production custody or escrow
- Real prepaid cards
- Bank-account aggregation
- Monitoring purchases outside the demo environment
- Weighted voting or quorum approval
- Production charitable disbursements
- Multiple simultaneous active pacts per user
