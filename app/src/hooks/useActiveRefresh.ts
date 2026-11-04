import { useCallback, useEffect, useRef } from 'react';
import { AppState } from 'react-native';
import { useFocusEffect } from 'expo-router';

const DEFAULT_INTERVAL_MS = 10_000;

/** Refreshes focused screens immediately, when the app returns, and while visible. */
export function useActiveRefresh(refresh: () => void | Promise<void>, intervalMs = DEFAULT_INTERVAL_MS) {
  const refreshRef = useRef(refresh);
  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useFocusEffect(useCallback(() => {
    let mounted = true;
    let running = false;
    let queued = false;

    const run = async () => {
      if (!mounted) return;
      if (running) {
        queued = true;
        return;
      }
      running = true;
      try {
        do {
          queued = false;
          try { await refreshRef.current(); } catch { /* Screens surface errors when useful; polling must keep running. */ }
        } while (mounted && queued);
      } finally {
        running = false;
      }
    };

    void run();
    const appStateSubscription = AppState.addEventListener('change', (state) => {
      if (state === 'active') void run();
    });
    const timer = setInterval(() => {
      if (AppState.currentState === 'active') void run();
    }, intervalMs);

    return () => {
      mounted = false;
      clearInterval(timer);
      appStateSubscription.remove();
    };
  }, [intervalMs]));
}
