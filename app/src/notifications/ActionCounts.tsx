import { createContext, type PropsWithChildren, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { AppState } from 'react-native';

import { getAccountabilityRequests, getInvitations } from '@/api/pacts';
import { getCurrentUser, getPaymentRequests } from '@/api/wallet';
import { getSessionToken } from '@/auth/session';

export type ActionCounts = {
  accountability: number;
  invitations: number;
  paymentRequests: number;
  total: number;
};

type ActionCountContextValue = {
  counts: ActionCounts;
  refresh: () => Promise<void>;
  reset: () => void;
};

const emptyCounts: ActionCounts = { accountability: 0, invitations: 0, paymentRequests: 0, total: 0 };
const ActionCountContext = createContext<ActionCountContextValue | null>(null);

export function ActionCountProvider({ children }: PropsWithChildren) {
  const [counts, setCounts] = useState<ActionCounts>(emptyCounts);
  const refreshing = useRef(false);
  const refreshAgain = useRef(false);

  const reset = useCallback(() => setCounts(emptyCounts), []);
  const refresh = useCallback(async () => {
    if (refreshing.current) {
      refreshAgain.current = true;
      return;
    }
    refreshing.current = true;
    try {
      do {
        refreshAgain.current = false;
        try {
          if (!await getSessionToken()) {
            reset();
            break;
          }
          const [current, invitations, accountabilityRequests, paymentRequests] = await Promise.all([
            getCurrentUser(),
            getInvitations(),
            getAccountabilityRequests(),
            getPaymentRequests(),
          ]);
          const userID = current.user.id;
          const now = Date.now();
          const next = {
            accountability: accountabilityRequests.filter((item) => item.status === 'ASSIGNED' && item.assignedPartnerId === userID && new Date(item.expiresAt).getTime() > now).length,
            invitations: invitations.filter((item) => item.status === 'PENDING' && item.partnerId === userID).length,
            paymentRequests: paymentRequests.filter((item) => item.status === 'PENDING' && item.recipient.id === userID && new Date(item.expiresAt).getTime() > now).length,
          };
          setCounts({ ...next, total: next.accountability + next.invitations + next.paymentRequests });
        } catch {
          // Keep the last known counts through brief network failures.
        }
      } while (refreshAgain.current);
    } finally {
      refreshing.current = false;
    }
  }, [reset]);

  useEffect(() => {
    const initialRefresh = setTimeout(() => void refresh(), 0);
    const subscription = AppState.addEventListener('change', (state) => {
      if (state === 'active') void refresh();
    });
    const timer = setInterval(() => {
      if (AppState.currentState === 'active') void refresh();
    }, 30_000);
    return () => {
      clearTimeout(initialRefresh);
      clearInterval(timer);
      subscription.remove();
    };
  }, [refresh]);

  const value = useMemo(() => ({ counts, refresh, reset }), [counts, refresh, reset]);
  return <ActionCountContext.Provider value={value}>{children}</ActionCountContext.Provider>;
}

export function useActionCounts() {
  const value = useContext(ActionCountContext);
  if (!value) throw new Error('useActionCounts must be used inside ActionCountProvider.');
  return value;
}
