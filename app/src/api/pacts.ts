import * as SecureStore from 'expo-secure-store';
import { apiRequest } from './client';
import type {
  AccountabilityCircle,
  AccountabilityInvitation,
  AccountabilityRequest,
  Pact,
  PactTermsInput,
} from './types';

export type LocalPactDraft = Partial<Omit<PactTermsInput, 'breakConsequence' | 'currency'>> & {
  serverPactId?: string;
  updatedAt: string;
};

const DRAFT_KEY = 'controlled.local-pact-draft';

export const getPacts = async () => (await apiRequest<{ pacts: Pact[] }>('/v1/pacts')).pacts;
export const getPact = async (id: string) => (await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}`)).pact;
export const createPact = async (terms: PactTermsInput) => (await apiRequest<{ pact: Pact }>('/v1/pacts', { body: terms, idempotent: true, method: 'POST' })).pact;
export const updatePact = async (id: string, terms: PactTermsInput) => (await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}`, { body: terms, method: 'PUT' })).pact;
export const confirmPact = async (id: string) => (await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/confirm`, { body: {}, method: 'POST' })).pact;
export const reservePactStake = async (id: string) => (await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/stake`, { body: {}, idempotent: true, method: 'POST' })).pact;
export const cancelPact = async (id: string) => (await apiRequest<{ pact: Pact }>(`/v1/pacts/${encodeURIComponent(id)}/cancel`, { body: {}, method: 'POST' })).pact;

export const getInvitations = async () => (await apiRequest<{ invitations: AccountabilityInvitation[] }>('/v1/accountability/invitations')).invitations;
export const invitePartner = async (partnerEmail: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>('/v1/accountability/invitations', { body: { partnerEmail }, idempotent: true, method: 'POST' })).invitation;
export const acceptInvitation = async (id: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>(`/v1/accountability/invitations/${encodeURIComponent(id)}/accept`, { body: {}, method: 'POST' })).invitation;
export const declineInvitation = async (id: string) => (await apiRequest<{ invitation: AccountabilityInvitation }>(`/v1/accountability/invitations/${encodeURIComponent(id)}/decline`, { body: {}, method: 'POST' })).invitation;

export const getCircles = async () => (await apiRequest<{ circles: AccountabilityCircle[] }>('/v1/accountability/circles')).circles;
export const createCircle = async (name: string, partnerIds: string[]) => (await apiRequest<{ circle: AccountabilityCircle }>('/v1/accountability/circles', { body: { name, partnerIds }, idempotent: true, method: 'POST' })).circle;

export const getAccountabilityRequests = async () => (await apiRequest<{ accountabilityRequests: AccountabilityRequest[] }>('/v1/accountability/requests')).accountabilityRequests;
export const getAccountabilityRequest = async (id: string) => (await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}`)).accountabilityRequest;
export const approveAccountabilityRequest = async (id: string, classification: 'EMERGENCY' | 'CONSCIOUS_BREAK') => (await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/approve`, { body: { classification }, method: 'POST' })).accountabilityRequest;
export const denyAccountabilityRequest = async (id: string) => (await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/deny`, { body: {}, method: 'POST' })).accountabilityRequest;
export const cancelAccountabilityRequest = async (id: string) => (await apiRequest<{ accountabilityRequest: AccountabilityRequest }>(`/v1/accountability/requests/${encodeURIComponent(id)}/cancel`, { body: {}, method: 'POST' })).accountabilityRequest;

export async function loadLocalPactDraft(): Promise<LocalPactDraft | null> {
  const value = await SecureStore.getItemAsync(DRAFT_KEY);
  if (!value) return null;
  try { return JSON.parse(value) as LocalPactDraft; } catch { return null; }
}

export async function saveLocalPactDraft(value: Omit<LocalPactDraft, 'updatedAt'>) {
  await SecureStore.setItemAsync(DRAFT_KEY, JSON.stringify({ ...value, updatedAt: new Date().toISOString() }));
}

export const clearLocalPactDraft = () => SecureStore.deleteItemAsync(DRAFT_KEY);
