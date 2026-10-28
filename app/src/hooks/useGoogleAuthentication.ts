import { router } from 'expo-router';
import { useState } from 'react';

import { authenticateWithGoogle } from '@/auth/google';

export function useGoogleAuthentication() {
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);

  const authenticate = async () => {
    if (loading) {
      return;
    }
    setError(undefined);
    setLoading(true);
    try {
      const result = await authenticateWithGoogle();
      if (result === 'authenticated') {
        router.replace('/onboarding');
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'Google sign-in failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return { authenticate, error, loading };
}
