import type { Href } from 'expo-router';

import { getCurrentUser } from '@/api/wallet';

// Where a signed-in user lands: first-time users still need to choose their name.
export async function signedInRoute(): Promise<Href> {
  try {
    const { user } = await getCurrentUser();
    return user.namePending ? '/hello' as Href : '/home';
  } catch {
    return '/home';
  }
}
