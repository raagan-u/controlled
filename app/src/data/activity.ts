import type { Transaction, Wallet } from '@/api/types';
import { formatUSD } from '@/api/wallet';

export type Activity = {
  amount: string;
  id: string;
  label: string;
  meta: string;
  outgoing?: boolean;
  requested?: boolean;
  neutral?: boolean;
  section: string;
  status: 'Completed' | 'Pending' | 'Failed';
};

const withdrawalTerminalTypes = new Set(['WITHDRAWAL_SETTLEMENT', 'WITHDRAWAL_RELEASE']);
const stakeMovementTypes = new Set(['PACT_STAKE_RESERVATION', 'PACT_STAKE_RELEASE', 'PACT_COMPLETION', 'PACT_CONSCIOUS_BREAK', 'PACT_STAKE_RECOVERY']);

export function visibleTransactions(transactions: Transaction[]) {
  const terminalWithdrawalIDs = new Set(
    transactions
      .filter((transaction) => withdrawalTerminalTypes.has(transaction.reference.type))
      .map((transaction) => transaction.reference.id),
  );
  return transactions.filter((transaction) =>
    transaction.reference.type !== 'WITHDRAWAL_RESERVATION'
    || !terminalWithdrawalIDs.has(transaction.reference.id),
  );
}

function sectionFor(value: string) {
  const date = new Date(value);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);
  if (date.toDateString() === today.toDateString()) return 'Today';
  if (date.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return date.toLocaleDateString('en-US', { day: 'numeric', month: 'short' });
}

function presentation(transaction: Transaction, outgoing: boolean) {
  const referenceType = transaction.reference.type;
  const reversed = transaction.reversed;
  if (reversed || referenceType === 'WITHDRAWAL_RELEASE') return { label: 'Withdraw to PayPal', status: 'Failed' as const };
  if (referenceType === 'FUNDING') return { label: 'Added from PayPal', status: 'Completed' as const };
  if (referenceType === 'INTERNAL_TRANSFER') {
    const counterparty = outgoing ? transaction.transferDetails?.recipient : transaction.transferDetails?.sender;
    return { label: counterparty ? `Payment ${outgoing ? 'to' : 'from'} ${counterparty.displayName}` : 'Internal transfer', status: 'Completed' as const };
  }
  if (referenceType === 'WITHDRAWAL_RESERVATION') return { label: 'Withdraw to PayPal', status: 'Pending' as const };
  if (referenceType === 'WITHDRAWAL_SETTLEMENT') return { label: 'Withdraw to PayPal', status: 'Completed' as const };
  if (referenceType === 'PACT_STAKE_RESERVATION') return { label: 'Stake reserved', status: 'Completed' as const };
  if (referenceType === 'PACT_STAKE_RELEASE' || referenceType === 'PACT_COMPLETION') return { label: 'Stake released', status: 'Completed' as const };
  if (referenceType === 'PACT_CONSCIOUS_BREAK') return { label: 'Stake moved to Commitment Reserve', status: 'Completed' as const };
  if (referenceType === 'PACT_STAKE_RECOVERY') return { label: 'Commitment Reserve recovered', status: 'Completed' as const };
  return { label: outgoing ? 'Money sent' : 'Money received', status: 'Completed' as const };
}

export function transactionToActivity(transaction: Transaction, wallet: Wallet): Activity {
  const ownAvailable = transaction.entries.find((entry) => entry.accountId === wallet.accounts.available);
  const ownEntry = ownAvailable ?? transaction.entries.find((entry) =>
    entry.accountId === wallet.accounts.pending || entry.accountId === wallet.accounts.commitmentReserve,
  ) ?? transaction.entries[0];
  const outgoing = ownEntry?.side === 'DEBIT';
  const amountMinor = ownEntry?.amount.amountMinor ?? 0;
  const view = presentation(transaction, outgoing);
  const neutral = stakeMovementTypes.has(transaction.reference.type);
  const time = new Date(transaction.occurredAt).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  const direction = transaction.reference.type === 'INTERNAL_TRANSFER' ? (outgoing ? 'Sent' : 'Received') : view.status;
  return {
    amount: formatUSD(amountMinor),
    id: transaction.id,
    label: view.label,
    meta: `${direction} · ${time}`,
    neutral,
    outgoing: neutral ? undefined : outgoing,
    section: sectionFor(transaction.occurredAt),
    status: view.status,
  };
}
