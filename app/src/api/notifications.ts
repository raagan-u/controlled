import { apiRequest } from './client';

export const registerPushDevice = (token: string, platform: 'android' | 'ios') =>
  apiRequest<void>('/v1/push-devices', { body: { platform, token }, method: 'POST' });

export const unregisterPushDevice = (token: string) =>
  apiRequest<void>('/v1/push-devices', { body: { token }, method: 'DELETE' });
