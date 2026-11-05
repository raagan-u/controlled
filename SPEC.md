# Controlled — Product specification

Status: implemented hackathon MVP

## 1. Product

Controlled is a mobile USD wallet that enforces financial commitments before money leaves. Users can fund a wallet, send or request money, withdraw to a PayPal destination, and create time-bound spending pacts backed by a financial stake and trusted accountability partners.

Without an active pact, outgoing transactions follow ordinary balance and validation rules. With one or more active pacts, every outgoing transaction is evaluated against all of them. Incoming funds never consume pact capacity.

The MVP is a sandbox proof of concept. It is not a bank, production custodial wallet, investment product, or money-transmission service.

## 2. User experience

### Authentication and onboarding

- Users register or sign in with email/password or native Google sign-in.
- Google ID tokens are verified by the backend before a Controlled session is issued.
- New users whose sign-up identity does not provide a chosen name complete a short name step.
- Sessions are stored in the device secure store and sent as bearer tokens.
- Password reset and email verification are not implemented in this MVP.

### Home

The Home screen shows:

- current available balance;
- shortcuts to fund, send/request, and withdraw;
- recent recipients;
- scheduled and active pact cards with distinct visual states and live progress; and
- recent transaction activity.

Balance, activity, recipients, and pact state refresh together when Home regains focus, when the app returns to the foreground, and every ten seconds while Home is focused and the app is active. Existing data remains visible during background refreshes.

### Send money

The user selects an existing Controlled recipient, enters an amount, and adds a required meaningful remark. Short labels and single words are allowed, but empty text and obvious keyboard noise are rejected. The remark is persisted and shown in transaction details.

The user can mark the transfer as a purchase. In purchase mode:

- the remark field asks for a specific product name/model;
- after both amount and product description are available, the client requests a Channel3-backed price check;
- a price check is skipped when the amount exceeds available balance;
- a matched result may show the best offer, merchant, product, savings, and verdict; and
- provider failure never blocks the payment.

The client waits 900 milliseconds after valid input settles before starting pact guidance or a price check. When both apply, it renders one combined loading status; matched price results and pact guidance still open automatically.

The purchase toggle affects guidance only; the transfer itself remains the same wallet operation.

### Request money

A requester selects another Controlled user, amount, and optional note. Requests expire after seven days. The recipient may pay or decline; the requester may cancel. Paying is an ordinary outgoing transfer and therefore passes through the payer's balance and active-pact policy.

If payment requires accountability approval, the payment request moves to `AWAITING_APPROVAL` and links to the resulting accountability request. It becomes paid only after the approved transfer executes.

### Fund and withdraw

Funding creates a provider order. The fake provider supports deterministic local confirmation; PayPal Sandbox returns an approval URL and captures the approved order through the API.

Withdrawal requires a meaningful reason, then reserves available funds into pending balance before provider payout. Successful settlement consumes the pending amount. Failed settlement releases it back to available and reverses corresponding pact usage. PayPal payouts can be reconciled by the API.

### Transaction details

Details show the ledger description, amount, time, parties, persisted transfer remark, and reversal state. If the transaction was executed through accountability approval, details also show that it was pact-breaking, whether it was an emergency or conscious break, the owner's reason, and the approving partner.

## 3. Wallet and ledger

Every user has one USD wallet with four balance categories:

- **Available** — spendable now
- **Pending** — reserved for in-flight withdrawal settlement
- **Staked** — reserved for scheduled or active pacts
- **Commitment Reserve** — user-owned funds retained after a conscious break and unavailable for spending, sending, or withdrawal

All state-changing wallet operations use balanced double-entry journal transactions. Money is represented as positive integer minor units; floating-point values are not used for authoritative amounts.

## 4. Pacts

A pact contains:

- owner;
- goal and spending intent;
- one total spending limit for the full period;
- positive stake amount;
- start and end timestamps;
- ordered accountability circle;
- `COMMITMENT_RESERVE` break consequence;
- status, outcome, version, progress, and emergency count.

Pact lifecycle:

```text
DRAFT -> AWAITING_STAKE -> SCHEDULED -> ACTIVE -> RESOLVED
   |          |               |
   +----------+---------------+-> CANCELLED
```

- Draft terms can be edited.
- New and edited pacts must span at least 24 hours from start to end.
- Confirmation validates the accountability circle and freezes terms.
- Stake reservation atomically moves funds from available to staked.
- A future start produces `SCHEDULED`; an arrived start produces `ACTIVE`.
- Scheduled pacts activate automatically.
- Active pacts resolve at their end as `KEPT` or `KEPT_WITH_EXCEPTIONS`, then release the stake.
- Draft, awaiting-stake, and scheduled pacts can be cancelled. Cancelling a scheduled pact releases its stake.
- Active pacts cannot be directly cancelled.
- A conscious break resolves the affected pact as `BROKEN` and moves the complete stake to Commitment Reserve.
- A later pact qualifies for recovery only when its scheduled duration is at least seven days and its stake was reserved after the reserve entry was created.
- A valid pact lasting from 24 hours to under seven days can enforce spending normally but cannot recover Commitment Reserve. Creation, review, and recovery screens explain this state.
- An eligible pact can recover reserve funds when it resolves `KEPT` or `KEPT_WITH_EXCEPTIONS`.
- Recovery is capped by the kept pact's stake and applied oldest-first across outstanding reserve stakes. Partial recovery is allowed.
- The kept pact's own stake release and any reserve recovery are committed atomically. Broken or cancelled pacts recover nothing.

The app saves an in-progress pact draft locally so the flow can be resumed. After successful creation it provides a direct route back Home.

## 5. Pact progress and overlapping policy

Successful internal transfers, payment-request payments, and withdrawal reservations consume pact capacity. Failed/reversed debits reverse their usage. Funding, incoming transfers, stake movement, and Commitment Reserve movement do not consume limits.

When multiple active pacts overlap:

- the transaction is checked against every active pact;
- one successful debit counts against every applicable pact;
- every pact must permit the debit;
- one accountability request snapshots every blocking pact; and
- an approval applies the selected outcome independently to each blocking pact.

The API returns used, remaining, and overage amounts plus the contributing usage records. The mobile app displays this progress on Home and pact details.

## 6. Accountability

### Relationships and circles

- An owner invites existing Controlled users.
- The invited user accepts or declines.
- A circle contains one or more distinct accepted partners in explicit order.
- The owner cannot include themselves.
- Circles are immutable; a membership/order change requires a new circle.

### Over-limit requests

An otherwise valid outgoing transaction that exceeds at least one active pact can create an accountability request when the owner provides a reason. No wallet funds or pact capacity are reserved while it waits.

Each request records:

- exact operation type, destination, amount, and owner reason;
- snapshots of every blocking pact;
- ordered reviewer assignments and decisions;
- required denial reasons;
- final classification, status, and executed operation ID.

One partner is active at a time. Each assignment lasts six hours; the overall request lasts 24 hours. A denial or timeout advances to the next eligible partner. If all partners deny, the request ends denied. The owner may cancel while pending.

| Decision | Transaction | Pact | Stake |
| --- | --- | --- | --- |
| Deny | remains blocked; may advance | continues | remains staked |
| Emergency | exact transaction executes | continues with exception | remains staked |
| Conscious break | exact transaction executes | resolves `BROKEN` | moves to Commitment Reserve |
| Expiry/cancel | cancelled | continues | remains staked |

Approval is single-use and scoped to the exact saved transaction. Before execution, Controlled atomically rechecks available balance and current pact state. An approval cannot override insufficient funds or unrelated/new policy blocks.

## 7. Guidance and price intelligence

The guidance layer supports:

- pact term suggestions from the user's goal, intent, balance, and history;
- transaction previews for proposed sends and withdrawals when at least one pact is active;
- pact progress insights; and
- factual accountability briefs.

Transaction previews are not requested when the user has no active pact, and the API independently returns `204 No Content` in that case. With overlapping active pacts, preview guidance uses the pact with the highest stake, then highest limit, then longest duration; deterministic enforcement still evaluates every active pact.

The preview summarizes outgoing transfers and withdrawal reservations from the last 30 days. It compares the most recent 15 days with the previous 15, identifies amount-weighted day/time and remark patterns, and reports whether at least three historical payments exist. The proposed remark is classified as potentially aligned, conflicting, exceptional, or unclear using only the pact goal/intent, transaction remark, and purchase flag. Generic or uncertain remarks remain neutral.

Forecast guidance begins only after three elapsed pact days. The stronger warning appears only when the proposed payment would bring usage to at least half the limit and projected end-of-pact spending would exceed the limit. Guidance remains neutral and concise unless verified pact or forecast facts justify a firmer, caring tone. Potentially aligned spending still counts toward the pact limit.

With AI disabled or unavailable, endpoints return deterministic rule-based guidance. With an OpenAI-compatible provider enabled, the backend sends a curated snapshot, requires structured JSON, validates the response, and falls back to rules on timeout, rate limiting, invalid output, or provider failure.

AI can explain and suggest. It cannot create/confirm a pact, move money, post ledger entries, classify an exception, select a reviewer, or approve a request.

Channel3 product search is optional. It is used only for purchase price checks and never participates in payment authorization.

## 8. Notifications and action state

The app registers an Expo push token after authentication. The backend sends best-effort notifications for:

- payment requests, cancellation, and decline;
- accountability invitations and responses;
- new approval assignments;
- approval, denial, reassignment, and cancellation; and
- money received.

Notification taps deep-link to the relevant existing screen. There is no separate inbox. In-app badges and refreshed server state remain the source of truth if push delivery fails.

## 9. Product invariants

1. Journal transactions balance.
2. Available balance cannot become negative.
3. Insufficient balance cannot be approved away.
4. A pact cannot schedule/activate until its full stake is reserved.
5. Confirmed pact terms are immutable.
6. Every active pact must permit an outgoing transaction.
7. Only the current assigned partner may decide an accountability request.
8. Denial requires a reason; approval requires a valid classification.
9. One approval grants only its exact saved operation.
10. A pact resolves once and a debit is counted once per pact.
11. Public money commands and create commands are idempotent.
12. Controlled never receives a broken pact's stake.
13. Provider, AI, shopping, and notification failures must not bypass ledger/policy rules.
14. All authoritative money is USD integer cents.
15. New pacts last at least 24 hours; reserve-recovery pacts last at least seven days.
16. Commitment Reserve recovery is oldest-first, cannot exceed eligible kept stake, and cannot recover a reserve entry created after the recovering pact's stake reservation.

## 10. Explicit MVP boundaries

- USD only
- PayPal Sandbox, not PayPal production
- No password recovery or email verification
- No production KYC/compliance or custody claims
- No cards, credit, lending, PYUSD, rewards, yield, or investment return
- No monitoring of spending outside Controlled
- No notification inbox
- No pagination in the mobile activity experience
