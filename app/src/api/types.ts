export type Money = { amountMinor: number; currency: 'USD' };
export type User = { createdAt: string; displayName: string; email: string; id: string };
export type Wallet = {
  accounts: { available: string; commitmentReserve: string; pending: string; staked?: string };
  balances: { available: Money; commitmentReserve: Money; pending: Money; staked?: Money };
  createdAt: string; currency: 'USD'; id: string; ownerId: string;
};
export type LedgerEntry = { accountId: string; accountKind: string; amount: Money; side: 'CREDIT' | 'DEBIT' };
export type Transaction = { description: string; entries: LedgerEntry[]; id: string; occurredAt: string; postedAt: string; reference: { id: string; type: string }; reversed: boolean };
export type FundingIntent = { amount: Money; id: string; status: 'AWAITING_CONFIRMATION' | 'COMPLETED' | 'FAILED' };
export type Withdrawal = { amount: Money; id: string; status: 'PENDING' | 'COMPLETED' | 'FAILED' };
export type Transfer = { amount: Money; duplicate?: boolean; id: string; status?: 'COMPLETED' | 'PENDING_APPROVAL' };
