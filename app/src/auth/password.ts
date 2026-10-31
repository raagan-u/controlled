import { createControlledSession } from '@/auth/session';

export type PasswordAuthenticationMode = 'login' | 'register';

export async function authenticateWithPassword(
  mode: PasswordAuthenticationMode,
  email: string,
  password: string,
): Promise<void> {
  await createControlledSession(`/v1/auth/password/${mode}`, {
    email: email.trim().toLowerCase(),
    password,
  });
}
