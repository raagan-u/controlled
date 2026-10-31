import { useCallback, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import type { Transaction, User, Wallet } from '@/api/types';
import { apiErrorMessage, getCurrentUser, getTransactions, getWallet } from '@/api/wallet';

type Overview = { transactions: Transaction[]; user: User; wallet: Wallet };

export function useWalletOverview() {
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const request = useRef(0);

  const refresh = useCallback(async () => {
    const current = ++request.current;
    setError('');
    setLoading(true);
    try {
      const [userResponse, walletResponse, transactionResponse] = await Promise.all([
        getCurrentUser(), getWallet(), getTransactions(),
      ]);
      if (request.current === current) {
        setData({ transactions: transactionResponse.transactions, user: userResponse.user, wallet: walletResponse.wallet });
      }
    } catch (cause) {
      if (request.current === current) setError(apiErrorMessage(cause));
    } finally {
      if (request.current === current) setLoading(false);
    }
  }, []);

  useFocusEffect(useCallback(() => {
    void refresh();
    return () => { request.current += 1; };
  }, [refresh]));

  return { data, error, loading, refresh };
}
