import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

export const sessionKey = 'controlled_session';
const localAPIURL = Platform.OS === 'android' ? 'http://10.0.2.2:8080' : 'http://127.0.0.1:8080';
export const apiURL = (process.env.EXPO_PUBLIC_API_URL ?? localAPIURL).replace(/\/$/, '');

type SessionResponse = {
  sessionToken?: string;
};

type APIError = {
  error?: {
    message?: string;
  };
};

export async function createControlledSession(path: string, body: object): Promise<void> {
  const response = await fetch(`${apiURL}${path}`, {
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
  });
  const payload = (await response.json().catch(() => ({}))) as SessionResponse & APIError;
  if (!response.ok) {
    throw new Error(payload.error?.message ?? 'Controlled could not complete authentication.');
  }
  if (!payload.sessionToken) {
    throw new Error('Controlled did not return a session token.');
  }
  await SecureStore.setItemAsync(sessionKey, payload.sessionToken);
}

export function getSessionToken(): Promise<string | null> {
  return SecureStore.getItemAsync(sessionKey);
}

export function clearSession(): Promise<void> {
  return SecureStore.deleteItemAsync(sessionKey);
}
