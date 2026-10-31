import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';
import type { User } from '@/api/types';
import { apiErrorMessage, createTransfer, formatUSD, getWallet, toMinorUnits } from '@/api/wallet';
import { AmountKeypad } from '@/components/AmountKeypad';
import { MoneyAmount } from '@/components/MoneyAmount';
import { RecipientPicker } from '@/components/RecipientPicker';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { TransferTabs } from '@/components/TransferTabs';
import { colors, fonts, radii } from '@/theme';

type Step = 'recipient' | 'amount' | 'success' | 'review';

export default function SendScreen() {
  const params = useLocalSearchParams<{ recipient?: string }>();
	const preset = typeof params.recipient === 'string' ? params.recipient.trim() : '';
  const [step, setStep] = useState<Step>('recipient');
  const [recipient, setRecipient] = useState<User | null>(null);
  const [amount, setAmount] = useState('');
  const [displayAmount, setDisplayAmount] = useState('');
  const [calculation, setCalculation] = useState<string | null>(null);
  const [remark, setRemark] = useState('');
  const [available, setAvailable] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const valid = amountMinor > 0 && !over && recipient !== null;

  useEffect(() => { void getWallet().then(({ wallet }) => setAvailable(wallet.balances.available.amountMinor)).catch((cause) => setError(apiErrorMessage(cause))); }, []);

  async function send() {
    if (!valid || submitting) return;
    setSubmitting(true); setError('');
    try {
		  if (!recipient) return;
      const result = await createTransfer(recipient.ctrldId, amountMinor, remark.trim() || undefined);
      setStep('accountabilityRequest' in result ? 'review' : 'success');
    } catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  if (step === 'success') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Done</Text>}><ResultState amount={amount} detail={`Sent to ${recipient?.displayName}${remark ? ` · ${remark}` : ''}`} title="Money sent" /></SheetScreen>;
  if (step === 'review') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Back home</Text>}><ResultState amount={amount} detail="This transfer needs approval from your accountability circle. We’ll update Activity after a decision." title="Approval requested" tone="pending" /></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><TransferTabs active="Send" /><RecipientPicker initialQuery={preset} onSelect={(user) => { setRecipient(user); setError(''); setStep('amount'); }} /></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><View><TransferTabs active="Send" /><Text style={styles.payee}>paying to <Text onPress={() => setStep('recipient')} style={styles.payeeStrong}>{recipient?.displayName}⌄</Text></Text></View><View style={styles.amountBlock}><MoneyAmount calculation={calculation} caption={`Available $${available === null ? '—' : formatUSD(available)}`} onCalculationPress={() => { if (calculation) { setAmount(calculation); setDisplayAmount(calculation); setCalculation(null); } }} tone={over ? 'danger' : 'default'} value={displayAmount} /><TextInput maxLength={80} onChangeText={setRemark} placeholder="Add a remark" placeholderTextColor={colors.textMuted} style={styles.remark} value={remark} />{over ? <Text style={styles.error}>More than your available balance.</Text> : error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting} key={`${submitting}-${error}`} label={submitting ? 'Sending…' : undefined} onConfirm={() => void send()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={setAmount} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, payee: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, payeeStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { flex: 1, justifyContent: 'center' }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' } });
