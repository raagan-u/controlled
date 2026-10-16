# Pay Pact

**An AI-assisted, PayPal-backed commitment wallet that helps people plan a realistic budget, put money behind the promise, and require trusted human approval before exceeding it.**

Pay Pact is part of the *From My Heart* series and is being developed for the PayPal AI Hackathon.

## The problem

People rarely break budgets because they cannot calculate. They break them because a plan made earlier becomes easy to renegotiate in the moment.

Most budgeting products report the failure after the money is gone. Pay Pact creates a meaningful pause before the promise is broken.

## The product

A user creates a daily or weekly spending plan and backs it with a financial stake. Spending within the plan remains straightforward. When a purchase would exceed the limit, the user cannot silently move the boundary.

Pay Pact first helps the user understand the impact and explore ways to preserve the commitment. If an override is still necessary, the request is routed to one of four trusted accountability partners using round-robin assignment.

- A legitimate emergency can be approved as an exception without being treated as an absolute failure.
- A conscious break can be approved, but the stake is forfeited to a neutral fund.
- Pay Pact never treats forfeited stakes as revenue.

## Vision

The long-term vision is a commitment-enforced wallet in which users can:

- Plan realistic daily or weekly budgets
- Pre-commit a spending allowance
- Back the commitment with a financial stake
- Spend through a controlled wallet experience
- Require trusted human approval for early access or an over-limit purchase
- Distinguish genuine emergencies from conscious breaks
- Learn from previous commitments without being shamed by them
- Return successful stakes and route forfeited stakes to a neutral destination

AI makes the wallet adaptive and humane. PayPal makes the commitment financially real. Trusted people handle exceptional decisions that software should not judge.

## What we are building for the hackathon

The hackathon project is a functional proof of concept, not a production wallet, escrow service, or prepaid-card product.

It will demonstrate one complete PayPal Sandbox journey:

1. Create a spending pact.
2. Add four accountability partners.
3. Fund a stake through PayPal Checkout.
4. Activate the pact after verified payment confirmation.
5. Attempt a purchase inside the controlled demo environment.
6. Stop an over-limit purchase and offer an override request.
7. Route the request to the next accountability partner.
8. Approve an emergency exception or a conscious break.
9. Return the stake after success or route it to a neutral-fund sandbox account after a conscious break.
10. Display an auditable timeline of every consequential decision and payment event.

For the prototype:

- Spending is limited to a controlled PayPal sandbox experience.
- Only the stake is placed in the sandbox Stake Vault; the weekly allowance is not held by Pay Pact.
- The neutral fund is represented by a sandbox account.
- Production custody, charitable-disbursement, and regulatory claims are explicitly out of scope.

## How AI is used

AI is a support layer around a deterministic financial core.

It may:

- Identify contradictions or missing information in a proposed pact
- Help a user construct a realistic commitment
- Explain the impact of an over-limit purchase
- Retrieve relevant lessons from previous pacts
- Suggest ways to preserve a commitment before requesting an override
- Produce a factual, neutral brief for the assigned accountability partner
- Help the user reflect on patterns after a pact ends

It may not:

- Calculate the authoritative balance
- Change an active pact
- Decide whether an emergency is legitimate
- Approve an override
- Move money independently

The governing principle is:

> AI proposes. Humans authorize. The domain engine validates. PayPal moves the money.

## Architecture direction

The MVP uses:

- A TypeScript web client
- A Go modular-monolith backend
- PostgreSQL for transactional state, jobs, and audit history
- PayPal Sandbox for checkout, capture, refund, webhooks, and forfeiture settlement
- An AI model with structured outputs, guarded tool calls, and lightweight commitment memory

All meaningful workflows are owned by the backend and can be exercised through API calls. The frontend is a client of those workflows, not their source of truth.

## Project status

Pay Pact is currently in the product and system-design phase.

- [Product specification](./SPEC.md)
- [Backend specification](./BACKEND_SPEC.md)

Implementation will proceed as a vertical slice using fake external providers first, followed by PayPal Sandbox integration, AI capabilities, and submission hardening.
