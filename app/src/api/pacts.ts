import * as SecureStore from 'expo-secure-store';
import { apiRequest } from './client';
import type {
  AccountabilityCircle,
  AccountabilityInvitation,
  AccountabilityRequest,
  Pact,
	StakeRecovery,
  PactTermsInput,
} from './types';

export type LocalPactDraft = Partial<Omit<PactTermsInput, 'breakConsequence' | 'currency'>> & {
  serverPactId?: string;
  updatedAt: string;
};

const DRAFT_KEY = 'controlled.local-pact-draft';

function normalizePact(pact: Pact): Pact {
  const limit = pact.terms.spendingLimit.amountMinor;
  const progressAvailable = Boolean(pact.progress);
  const usedMinor = pact.progress?.usedMinor ?? 0;
  return {
    ...pact,
    progressAvailable,
    progress: {
      overageMinor: pact.progress?.overageMinor ?? Math.max(0, usedMinor - limit),
      remainingMinor: pact.progress?.remainingMinor ?? Math.max(0, limit - usedMinor),
      usage: pact.progress?.usage ?? [],
      usedMinor,
    },
  };
}

function normalizeAccountabilityRequest(request: AccountabilityRequest): AccountabilityRequest {
  const blockingPacts = (request.blockingPacts ?? []).map((pact) => ({
    ...pact,
    goal: pact.goal || 'Spending limit',
    intent: pact.intent || 'This payment would exceed the pact limit.',
  }));
  const reviewers = request.reviewers?.length
    ? request.reviewers
    : request.assignedPartnerId
      ? [{ partnerId: request.assignedPartnerId, position: 0, status: 'ACTIVE' as const }]
      : [];
  return { ...request, blockingPacts, reviewers };
}

export const getPacts = async () => (await apiRequest<{ pacts: Pact[] }>('/v1/pacts')).pacts.map(normalizePact);
export const getPact = async (id: string) => normalizePact((await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}`)).pact);
export const createPact = async (terms: PactTermsInput) => normalizePact((await apiRequest<{ pact: Pact }>('/v1/pacts', { body: terms, idempotent: true, method: 'POST' })).pact);
export const updatePact = async (id: string, terms: PactTermsInput) => normalizePact((await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}`, { body: terms, method: 'PUT' })).pact);
export const confirmPact = async (id: string) => normalizePact((await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/confirm`, { body: {}, method: 'POST' })).pact);
export const reservePactStake = async (id: string) => normalizePact((await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/stake`, { body: {}, idempotent: true, method: 'POST' })).pact);
export const cancelPact = async (id: string) => normalizePact((await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/cancel`, { body: {}, method: 'POST' })).pact);
export const getStakeRecovery = () => apiRequest<StakeRecovery>('/v1/pacts/stake-recovery');

export const getInvitations = async () => (await apiRequest<{ invitations: AccountabilityInvitation[] }>('/v1/accountability/invitations')).invitations;
export const invitePartner = async (partnerEmail: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>('/v1/accountability/invitations', { body: { partnerEmail }, idempotent: true, method: 'POST' })).invitation;
export const acceptInvitation = async (id: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>(`/v1/accountability/invitations/${encodeURIComponent(id)}/accept`, { body: {}, method: 'POST' })).invitation;
export const declineInvitation = async (id: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>(`/v1/accountability/invitations/${encodeURIComponent(id)}/decline`, { body: {}, method: 'POST' })).invitation;

export const getCircles = async () => (await apiRequest<{ circles: AccountabilityCircle[] }>('/v1/accountability/circles')).circles;
export const createCircle = async (name: string, partnerIds: string[]) => (await apiRequest<{ circle: AccountabilityCircle }>('/v1/accountability/circles', { body: { name, partnerIds }, idempotent: true, method: 'POST' })).circle;

export const getAccountabilityRequests = async () => (await apiRequest<{ accountabilityRequests: AccountabilityRequest[] }>('/v1/accountability/requests')).accountabilityRequests.map(normalizeAccountabilityRequest);
export const getAccountabilityRequest = async (id: string) => normalizeAccountabilityRequest((await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}`)).accountabilityRequest);
export const approveAccountabilityRequest = async (id: string, classification: 'EMERGENCY' | 'CONSCIOUS_BREAK') => normalizeAccountabilityRequest((await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/approve`, { body: { classification }, method: 'POST' })).accountabilityRequest);
export const denyAccountabilityRequest = async (id: string, reason: string) => normalizeAccountabilityRequest((await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/deny`, { body: { reason }, method: 'POST' })).accountabilityRequest);
export const cancelAccountabilityRequest = async (id: string) => normalizeAccountabilityRequest((await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/cancel`, { body: {}, method: 'POST' })).accountabilityRequest);

export async function loadLocalPactDraft(): Promise<LocalPactDraft | null> {
  const value = await SecureStore.getItemAsync(DRAFT_KEY);
  if (!value) return null;
  try { return JSON.parse(value) as LocalPactDraft; } catch { return null; }
}

export async function saveLocalPactDraft(value: Omit<LocalPactDraft, 'updatedAt'>) {
  await SecureStore.setItemAsync(DRAFT_KEY, JSON.stringify({ ...value, updatedAt: new Date().toISOString() }));
}

export const clearLocalPactDraft = () => SecureStore.deleteItemAsync(DRAFT_KEY);
