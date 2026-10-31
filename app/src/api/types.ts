export type Money = { amountMinor: number; currency: 'USD' };
export type User = { createdAt: string; ctrldId: string; displayName: string; email: string; id: string };
export type Wallet = {
  accounts: { available: string; commitmentReserve: string; pending: string; staked?: string };
  balances: { available: Money; commitmentReserve: Money; pending: Money; staked?: Money };
  createdAt: string; currency: 'USD'; id: string; ownerId: string;
};
export type LedgerEntry = { accountId: string; accountKind: string; amount: Money; side: 'CREDIT' | 'DEBIT' };
export type Transaction = { description: string; entries: LedgerEntry[]; id: string; occurredAt: string; postedAt: string; reference: { id: string; type: string }; reversed: boolean; transferDetails?: { paymentRequestId?: string; recipient: PaymentRequestParty; remark?: string; sender: PaymentRequestParty } };
export type FundingIntent = {
  amount: Money;
	grossAmount?: Money;
  approvalUrl?: string;
  completedAt?: string;
  duplicate?: boolean;
  id: string;
  ledgerTransactionId?: string;
  status: 'CREATED' | 'AWAITING_CONFIRMATION' | 'COMPLETED' | 'FAILED';
};
export type Withdrawal = {
  amount: Money;
  completedAt?: string;
  destination: string;
  duplicate?: boolean;
  id: string;
  settlementTransactionId?: string;
  status: 'RESERVED' | 'PENDING' | 'COMPLETED' | 'FAILED';
};
export type Transfer = {
  amount: Money;
  duplicate?: boolean;
  id: string;
  ledgerTransactionId: string;
  recipientEmail: string;
  recipientId: string;
};
export type PaymentRequestParty = Pick<User, 'id' | 'ctrldId' | 'displayName' | 'email'>;
export type PaymentRequest = {
  amountMinor: number;
	accountabilityRequestId?: string;
  createdAt: string;
  currency: 'USD';
  duplicate: boolean;
	expiresAt: string;
  id: string;
  note: string;
	recipient: PaymentRequestParty;
	requester: PaymentRequestParty;
	resolvedAt?: string;
	status: 'PENDING' | 'PROCESSING' | 'AWAITING_APPROVAL' | 'PAID' | 'DECLINED' | 'CANCELLED' | 'EXPIRED';
	transferId?: string;
	updatedAt: string;
};
export type AccountabilityRequest = {
  assignedPartnerId?: string;
  amountMinor: number;
  blockingPacts: AccountabilityRequestPact[];
  classification?: 'EMERGENCY' | 'CONSCIOUS_BREAK';
  createdAt: string;
  currency: 'USD';
  destination: string;
  executedOperationId?: string;
  expiresAt: string;
  id: string;
  intentId: string;
  operationType: 'INTERNAL_TRANSFER' | 'WITHDRAWAL';
  ownerId: string;
  reason: string;
  resolvedAt?: string;
  reviewers: AccountabilityReviewer[];
  status: 'ASSIGNED' | 'APPROVED' | 'DENIED' | 'EXPIRED' | 'CANCELLED' | 'FAILED';
};

export type AccountabilityRequestPact = {
  goal: string;
  intent: string;
  limitMinor: number;
  overageMinor: number;
  pactId: string;
  proposedMinor: number;
  usedMinor: number;
};

export type AccountabilityReviewer = {
  assignedAt?: string;
  decision?: 'APPROVE_EMERGENCY' | 'APPROVE_CONSCIOUS_BREAK' | 'DENY';
  expiresAt?: string;
  partnerId: string;
  position: number;
  respondedAt?: string;
  status: 'WAITING' | 'ACTIVE' | 'RESPONDED' | 'TIMED_OUT' | 'SUPERSEDED';
};

export type AccountabilityInvitation = {
  createdAt: string;
  duplicate: boolean;
  id: string;
  ownerId: string;
  partnerId: string;
  resolvedAt?: string;
  status: 'PENDING' | 'ACCEPTED' | 'DECLINED';
};

export type AccountabilityCircle = {
  createdAt: string;
  duplicate: boolean;
  id: string;
  name: string;
  partnerIds: string[];
};

export type PactStatus = 'DRAFT' | 'AWAITING_STAKE' | 'SCHEDULED' | 'ACTIVE' | 'RESOLVED' | 'CANCELLED';
export type PactOutcome = 'NONE' | 'KEPT' | 'KEPT_WITH_EXCEPTIONS' | 'BROKEN' | 'CANCELLED';
export type PactUsage = {
  amountMinor: number;
  countedAt: string;
  counterpartyCtrldId?: string;
  counterpartyName?: string;
  currency: 'USD';
  destination?: string;
  operationId: string;
  operationType: 'INTERNAL_TRANSFER' | 'WITHDRAWAL';
  reversedAt?: string;
  status: 'COUNTED' | 'REVERSED';
};
export type Pact = {
  duplicate: boolean;
  emergencyCount: number;
  id: string;
  outcome: PactOutcome;
  progress: { overageMinor: number; remainingMinor: number; usage: PactUsage[]; usedMinor: number };
  stakeReservationId?: string;
  status: PactStatus;
  terms: {
    accountabilityCircleId: string;
    breakConsequence: 'COMMITMENT_RESERVE';
    endsAt: string;
    goal: string;
    intent: string;
    spendingLimit: Money;
    stake: Money;
    startsAt: string;
  };
  version: number;
};

export type PactTermsInput = {
  accountabilityCircleId: string;
  breakConsequence: 'COMMITMENT_RESERVE';
  currency: 'USD';
  endsAt: string;
  goal: string;
  intent: string;
  spendingLimitMinor: number;
  stakeMinor: number;
  startsAt: string;
};
