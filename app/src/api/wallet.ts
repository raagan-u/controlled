import { apiRequest } from './client';
import type { AccountabilityRequest, FundingIntent, PaymentRequest, Transaction, Transfer, User, Wallet, Withdrawal } from './types';
import { clearSession } from '@/auth/session';

export const getCurrentUser = () => apiRequest<{ user: User }>('/v1/users/current');
export const getRecipients = (query = '') => apiRequest<{ users: User[] }>(`/v1/users/recipients${query.trim() ? `?query=${encodeURIComponent(query.trim())}` : ''}`);
export const getWallet = () => apiRequest<{ wallet: Wallet }>('/v1/wallet');
export const getTransactions = () => apiRequest<{ transactions: Transaction[] }>('/v1/wallet/transactions');
export const getTransaction = (id: string) => apiRequest<{ transaction: Transaction }>(`/v1/wallet/transactions/${encodeURIComponent(id)}`);
export const createFundingIntent = async (amountMinor: number) => (await apiRequest<{ fundingIntent: FundingIntent }>('/v1/wallet/funding-intents', { body: { amountMinor, currency: 'USD' }, idempotent: true, method: 'POST' })).fundingIntent;
export const confirmDemoFunding = async (id: string) => (await apiRequest<{ fundingIntent: FundingIntent }>(`/v1/demo/funding-intents/${encodeURIComponent(id)}/confirm`, { body: {}, method: 'POST' })).fundingIntent;
export const createTransfer = async (recipient: string, amountMinor: number, reason?: string) => apiRequest<{ accountabilityRequest: AccountabilityRequest } | { transfer: Transfer }>('/v1/wallet/internal-transfers', { body: { amountMinor, currency: 'USD', recipientEmail: recipient, ...(reason ? { reason } : {}) }, idempotent: true, method: 'POST' });
export const createPaymentRequest = async (recipientId: string, amountMinor: number, note?: string) => (await apiRequest<{ paymentRequest: PaymentRequest }>('/v1/wallet/payment-requests', { body: { amountMinor, currency: 'USD', recipientId, ...(note ? { note } : {}) }, idempotent: true, method: 'POST' })).paymentRequest;
export const createWithdrawal = async (destination: string, amountMinor: number, reason?: string) => apiRequest<{ accountabilityRequest: AccountabilityRequest } | { withdrawal: Withdrawal }>('/v1/wallet/withdrawals', { body: { amountMinor, currency: 'USD', destination, ...(reason ? { reason } : {}) }, idempotent: true, method: 'POST' });
export const settleDemoWithdrawal = async (id: string, succeeded: boolean) => (await apiRequest<{ withdrawal: Withdrawal }>(`/v1/demo/withdrawals/${encodeURIComponent(id)}/settle`, { body: { succeeded }, method: 'POST' })).withdrawal;
export async function signOut() { try { await apiRequest<void>('/v1/sessions/current', { method: 'DELETE' }); } finally { await clearSession(); } }

export function toMinorUnits(value: string): number { return Math.round(Number(value) * 100); }
export function formatUSD(amountMinor: number): string { return (amountMinor / 100).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }); }

export function apiErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Controlled could not complete this request.';
}
