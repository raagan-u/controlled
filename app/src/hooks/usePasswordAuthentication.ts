import { router } from 'expo-router';
import { useState } from 'react';

import {
  authenticateWithPassword,
  PasswordAuthenticationMode,
} from '@/auth/password';
import { AuthValues } from '@/components/AuthFormScreen';

export function usePasswordAuthentication(mode: PasswordAuthenticationMode) {
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);

  const authenticate = async (values: AuthValues) => {
    if (loading) {
      return;
    }
    setError(undefined);
    setLoading(true);
    try {
		  await authenticateWithPassword(mode, values.email, values.password, values.displayName);
		  router.replace('/home');
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Authentication failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return { authenticate, error, loading };
}
