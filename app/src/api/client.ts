import { apiURL, clearSession, getSessionToken } from '@/auth/session';

type RequestOptions = { body?: object; idempotent?: boolean; method?: 'DELETE' | 'GET' | 'POST' | 'PUT' };
type ErrorPayload = { error?: { code?: string; message?: string } };

export class APIError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message); this.name = 'APIError'; }
}

function idempotencyKey() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = await getSessionToken();
  if (!token) throw new APIError('missing_session', 'Please sign in again.', 401);
  const headers: Record<string, string> = { Accept: 'application/json', Authorization: `Bearer ${token}` };
  if (options.body) headers['Content-Type'] = 'application/json';
  if (options.idempotent) headers['Idempotency-Key'] = idempotencyKey();
  const response = await fetch(`${apiURL}${path}`, { body: options.body ? JSON.stringify(options.body) : undefined, headers, method: options.method ?? 'GET' });
  if (response.status === 401) await clearSession();
  if (!response.ok) {
    const payload = (await response.json().catch(() => ({}))) as ErrorPayload;
    throw new APIError(payload.error?.code ?? 'request_failed', payload.error?.message ?? 'Controlled could not complete this request.', response.status);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
