import { createControlledSession } from '@/auth/session';

export type PasswordAuthenticationMode = 'login' | 'register';

export async function authenticateWithPassword(
  mode: PasswordAuthenticationMode,
  email: string,
  password: string,
	displayName?: string,
): Promise<void> {
  await createControlledSession(`/v1/auth/password/${mode}`, {
    email: email.trim().toLowerCase(),
    password,
		...(mode === 'register' ? { displayName: displayName?.trim() } : {}),
  });
}
