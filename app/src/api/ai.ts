import { apiRequest } from './client';

export type GuidanceTone = 'NEUTRAL' | 'POSITIVE' | 'CAUTION' | 'CRITICAL';
export type GuidanceSource = 'AI' | 'RULES';

export type Guidance = {
  facts: { label: string; value: string }[];
  generatedAt: string;
  headline: string;
  kind: 'PACT_CREATION' | 'PACT_PROGRESS' | 'ACCOUNTABILITY_BRIEF' | 'TRANSACTION_PREVIEW';
  message: string;
  source: GuidanceSource;
  suggestedTerms?: { durationDays: number; limitMinor: number; stakeMinor: number };
  tone: GuidanceTone;
};

export type TransactionAssessment = {
  classifierVersion: string;
  confidence: 'HIGH' | 'MEDIUM' | 'LOW';
  forecastWarning: boolean;
  guidance: Guidance;
  id?: string;
  pattern: {
    change: 'DECREASING' | 'INCREASING' | 'NEW_ACTIVITY' | 'STABLE';
		alignedCount: number;
		conflictingCount: number;
		exceptionalCount: number;
    frequentReasons?: { amountMinor: number; count: number; reason: string }[];
    hasSufficientHistory: boolean;
    peakDay?: string;
    peakTime?: string;
    previousCount: number;
    previousSpentMinor: number;
    recentCount: number;
    recentSpentMinor: number;
		unclearCount: number;
  };
  primaryPactId?: string;
  projectedMinor?: number;
  relationship: 'ALIGNED' | 'CONFLICTING' | 'EXCEPTIONAL' | 'UNCLEAR';
};

const guidance = (path: string) => apiRequest<{ guidance: Guidance }>(path).then((response) => response.guidance);

export const getPactSuggestion = (goal: string, intent: string) =>
  apiRequest<{ guidance: Guidance }>('/v1/ai/pact-suggestions', { body: { goal, intent }, method: 'POST' }).then((response) => response.guidance);

export const getPactInsight = (id: string) => guidance(`/v1/ai/pacts/${encodeURIComponent(id)}/insight`);

export const getAccountabilityBrief = (id: string) => guidance(`/v1/ai/accountability/${encodeURIComponent(id)}/brief`);

export const assessTransaction = (amountMinor: number, remark: string, isPurchase: boolean) =>
  apiRequest<{ assessment: TransactionAssessment }>('/v1/ai/transaction-assessments', {
    body: { amountMinor, isPurchase, remark, timezoneOffsetMinutes: new Date().getTimezoneOffset() }, method: 'POST', timeoutMs: 20_000,
  }).then((response) => response.assessment);
