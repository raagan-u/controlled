import { router, type Href } from 'expo-router';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type { Withdrawal } from '@/api/types';
import { apiErrorMessage, createWithdrawal, formatUSD, getCurrentUser, getWallet, settleDemoWithdrawal, toMinorUnits } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { colors, fonts } from '@/theme';

type Step = 'amount' | 'processing' | 'success' | 'failed' | 'review';

export default function WithdrawScreen() {
  const [amount, setAmount] = useState('');
  const [displayAmount, setDisplayAmount] = useState('');
  const [calculation, setCalculation] = useState<string | null>(null);
  const [available, setAvailable] = useState<number | null>(null);
  const [destination, setDestination] = useState('PayPal');
  const [withdrawal, setWithdrawal] = useState<Withdrawal | null>(null);
  const [step, setStep] = useState<Step>('amount');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const valid = amountMinor > 0 && !over;

  useEffect(() => { void Promise.all([getWallet(), getCurrentUser()]).then(([{ wallet }, { user }]) => { setAvailable(wallet.balances.available.amountMinor); setDestination(`PayPal · ${user.email}`); }).catch((cause) => setError(apiErrorMessage(cause))); }, []);

  async function withdraw() {
    if (!valid || submitting) return;
    setSubmitting(true); setError('');
    try {
      const result = await createWithdrawal(destination, amountMinor);
      if ('accountabilityRequest' in result) setStep('review');
      else { setWithdrawal(result.withdrawal); setStep('processing'); }
    } catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  async function settle(succeeded: boolean) {
    if (!withdrawal || submitting) return;
    setSubmitting(true); setError('');
    try {
      const settled = await settleDemoWithdrawal(withdrawal.id, succeeded);
      setWithdrawal(settled);
      setStep(settled.status === 'COMPLETED' ? 'success' : 'failed');
    } catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  if (step === 'success') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="The payout completed and your Activity has been updated." title="Withdrawal complete" /></SheetScreen>;
  if (step === 'failed') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="The payout failed, so the reserved money was returned to your available balance." title="Withdrawal failed" tone="failed" /></SheetScreen>;
  if (step === 'review') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="This withdrawal needs approval from your accountability circle." title="Approval requested" tone="pending" /></SheetScreen>;
  if (step === 'processing') return <SheetScreen footer={<View style={styles.settlement}><ActionButton disabled={submitting} label="Simulate failed payout" onPress={() => void settle(false)} tone="danger" /><ActionButton disabled={submitting} label={submitting ? 'Settling…' : 'Complete payout'} onPress={() => void settle(true)} /></View>}><ResultState amount={amount} detail={error || 'The money is reserved and the payout is pending settlement.'} title="It’s on its way" tone="pending" /></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><FlowTabs active="Withdraw" onFund={() => router.replace('/fund' as Href)} onWithdraw={() => undefined} /><Text style={styles.destination}>move funds to <Text style={styles.destinationStrong}>{destination}⌄</Text></Text><View style={styles.amountBlock}><MoneyAmount calculation={calculation} onCalculationPress={() => { if (calculation) { setAmount(calculation); setDisplayAmount(calculation); setCalculation(null); } }} tone={over ? 'danger' : 'default'} value={displayAmount} />{calculation ? null : <View style={styles.availableRow}><Text style={[styles.available, over && styles.over]}>{over ? 'More than available' : 'Available'}   ${available === null ? '—' : formatUSD(available)}</Text><Text onPress={() => available !== null && setAmount((available / 100).toFixed(2))} style={styles.max}>Max</Text></View>}<Text style={styles.reserve}>Staked and reserved money can’t be withdrawn.</Text>{error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting} key={`${submitting}-${error}`} label={submitting ? 'Creating withdrawal…' : undefined} onConfirm={() => void withdraw()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={setAmount} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, destination: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, destinationStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { alignItems: 'center', flex: 1, justifyContent: 'center' }, availableRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'center', marginTop: -22 }, available: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14 }, over: { color: colors.redText }, max: { backgroundColor: colors.softButton, borderRadius: 8, color: colors.blue, fontFamily: fonts.bold, fontSize: 12, marginLeft: 10, overflow: 'hidden', paddingHorizontal: 10, paddingVertical: 6 }, reserve: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 10 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, settlement: { gap: 8 } });
