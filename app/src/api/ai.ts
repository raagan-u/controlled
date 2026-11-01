import { apiRequest } from './client';

export type GuidanceTone = 'NEUTRAL' | 'POSITIVE' | 'CAUTION' | 'CRITICAL';
export type GuidanceSource = 'AI' | 'RULES';

export type Guidance = {
  facts: { label: string; value: string }[];
  generatedAt: string;
  headline: string;
  kind: 'PACT_CREATION' | 'PACT_PROGRESS' | 'ACCOUNTABILITY_BRIEF';
  message: string;
  source: GuidanceSource;
  suggestedTerms?: { durationDays: number; limitMinor: number; stakeMinor: number };
  tone: GuidanceTone;
};

const guidance = (path: string) => apiRequest<{ guidance: Guidance }>(path).then((response) => response.guidance);

export const getPactSuggestion = (goal: string, intent: string) =>
  apiRequest<{ guidance: Guidance }>('/v1/ai/pact-suggestions', { body: { goal, intent }, method: 'POST' }).then((response) => response.guidance);

export const getPactInsight = (id: string) => guidance(`/v1/ai/pacts/${encodeURIComponent(id)}/insight`);

export const getAccountabilityBrief = (id: string) => guidance(`/v1/ai/accountability/${encodeURIComponent(id)}/brief`);
