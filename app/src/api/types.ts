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
  assignedPartnerId: string;
  createdAt: string;
  duplicate: boolean;
  expiresAt: string;
  id: string;
  intentId: string;
  status: string;
};
