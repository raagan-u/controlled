import { apiRequest } from './client';
import type { FundingIntent, Transaction, Transfer, User, Wallet, Withdrawal } from './types';
import { clearSession } from '@/auth/session';

export const getCurrentUser = () => apiRequest<{ user: User }>('/v1/users/current');
export const getWallet = () => apiRequest<{ wallet: Wallet }>('/v1/wallet');
export const getTransactions = () => apiRequest<{ transactions: Transaction[] }>('/v1/wallet/transactions');
export const getTransaction = (id: string) => apiRequest<{ transaction: Transaction }>(`/v1/wallet/transactions/${encodeURIComponent(id)}`);
export const createFundingIntent = (amountMinor: number) => apiRequest<FundingIntent>('/v1/wallet/funding-intents', { body: { amountMinor, currency: 'USD' }, idempotent: true, method: 'POST' });
export const confirmDemoFunding = (id: string) => apiRequest<FundingIntent>(`/v1/demo/funding-intents/${encodeURIComponent(id)}/confirm`, { body: {}, method: 'POST' });
export const createTransfer = (recipientEmail: string, amountMinor: number, reason?: string) => apiRequest<Transfer>('/v1/wallet/internal-transfers', { body: { amountMinor, currency: 'USD', recipientEmail, ...(reason ? { reason } : {}) }, idempotent: true, method: 'POST' });
export const createWithdrawal = (destination: string, amountMinor: number, reason?: string) => apiRequest<Withdrawal>('/v1/wallet/withdrawals', { body: { amountMinor, currency: 'USD', destination, ...(reason ? { reason } : {}) }, idempotent: true, method: 'POST' });
export const settleDemoWithdrawal = (id: string, succeeded: boolean) => apiRequest<Withdrawal>(`/v1/demo/withdrawals/${encodeURIComponent(id)}/settle`, { body: { succeeded }, method: 'POST' });
export async function signOut() { try { await apiRequest<void>('/v1/sessions/current', { method: 'DELETE' }); } finally { await clearSession(); } }

export function toMinorUnits(value: string): number { return Math.round(Number(value) * 100); }
export function formatUSD(amountMinor: number): string { return (amountMinor / 100).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }); }
