import { useCallback, useState } from 'react';
import { useFocusEffect, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import type { Transaction, Wallet } from '@/api/types';
import { apiErrorMessage, getTransaction, getWallet } from '@/api/wallet';
import { SheetScreen } from '@/components/SheetScreen';
import { transactionToActivity } from '@/data/activity';
import { colors, fonts, radii } from '@/theme';

export default function TransactionDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [data, setData] = useState<{ transaction: Transaction; wallet: Wallet } | null>(null);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [transactionResponse, walletResponse] = await Promise.all([getTransaction(id), getWallet()]);
      setError('');
      setData({ transaction: transactionResponse.transaction, wallet: walletResponse.wallet });
    } catch (cause) { setError(apiErrorMessage(cause)); }
  }, [id]);

  useFocusEffect(useCallback(() => { void load(); }, [load]));

  if (!data && !error) return <SheetScreen><ActivityIndicator color={colors.blue} style={styles.loader} /></SheetScreen>;
  if (!data) return <SheetScreen><Pressable onPress={() => void load()} style={styles.error}><Text style={styles.title}>Couldn’t load transaction</Text><Text style={styles.errorCopy}>{error}{'\n'}Tap to try again.</Text></Pressable></SheetScreen>;

  const item = transactionToActivity(data.transaction, data.wallet);
  const date = new Date(data.transaction.occurredAt).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' });
  const transfer = data.transaction.transferDetails;
	const transactionType = transfer?.paymentRequestId
		? 'Money request payment'
		: data.transaction.reference.type === 'INTERNAL_TRANSFER'
			? 'Transfer'
			: data.transaction.reference.type.replaceAll('_', ' ');
  const details = [
	['Status', item.status],
	...(transfer ? [['Paid by', `${transfer.sender.displayName} · ${transfer.sender.ctrldId}`], ['Received by', `${transfer.recipient.displayName} · ${transfer.recipient.ctrldId}`]] : []),
	...(transfer?.remark ? [['Remark', transfer.remark]] : []),
	['Date', date],
	['Transaction ID', data.transaction.id],
	['Type', transactionType],
  ];
  return <SheetScreen scroll><View style={[styles.statusIcon, item.status === 'Failed' && styles.failedIcon]}><Text style={[styles.check, item.status === 'Failed' && styles.failedCheck]}>{item.status === 'Failed' ? '×' : '✓'}</Text></View><Text style={[styles.title, item.status === 'Failed' && styles.failedTitle]}>{item.status}</Text><Text style={styles.amount}>{item.outgoing ? '−' : item.neutral ? '' : '+'}${item.amount}</Text><Text style={styles.label}>{item.label}</Text><View style={styles.card}>{details.map(([key, value]) => <View key={key} style={styles.row}><Text style={styles.key}>{key}</Text><Text style={styles.value}>{value}</Text></View>)}</View></SheetScreen>;
}

const styles = StyleSheet.create({ loader: { marginTop: 100 }, error: { alignItems: 'center', justifyContent: 'center', marginTop: 100, padding: 24 }, errorCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginTop: 8, textAlign: 'center' }, statusIcon: { alignItems: 'center', alignSelf: 'center', backgroundColor: colors.greenTint, borderRadius: 32, height: 64, justifyContent: 'center', marginTop: 24, width: 64 }, failedIcon: { backgroundColor: colors.redTint }, check: { color: colors.greenText, fontFamily: fonts.bold, fontSize: 30 }, failedCheck: { color: colors.redText }, title: { color: colors.greenText, fontFamily: fonts.bold, fontSize: 15, marginTop: 14, textAlign: 'center' }, failedTitle: { color: colors.redText }, amount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 48, letterSpacing: -1.5, marginTop: 8, textAlign: 'center' }, label: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15, marginTop: 4, textAlign: 'center' }, card: { backgroundColor: colors.surface, borderRadius: radii.lg, marginTop: 36, padding: 16 }, row: { borderBottomColor: colors.divider, borderBottomWidth: 1, flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 14 }, key: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14 }, value: { color: colors.navy, fontFamily: fonts.bold, fontSize: 14, maxWidth: '60%', textAlign: 'right' } });
