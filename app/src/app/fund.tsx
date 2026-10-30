import { router, type Href } from 'expo-router';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import * as Linking from 'expo-linking';
import type { FundingIntent } from '@/api/types';
import { apiErrorMessage, captureFunding, createFundingIntent, formatUSD, getWallet, toMinorUnits } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { colors, fonts, radii } from '@/theme';

type Step = 'amount' | 'paypal' | 'processing' | 'success' | 'failed';

export default function FundScreen() {
  const [amount, setAmount] = useState('');
  const [displayAmount, setDisplayAmount] = useState('');
  const [calculation, setCalculation] = useState<string | null>(null);
  const [available, setAvailable] = useState<number | null>(null);
  const [intent, setIntent] = useState<FundingIntent | null>(null);
  const [step, setStep] = useState<Step>('amount');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const valid = Number(amount) > 0 && toMinorUnits(amount) > 0;

  useEffect(() => { void getWallet().then(({ wallet }) => setAvailable(wallet.balances.available.amountMinor)).catch((cause) => setError(apiErrorMessage(cause))); }, []);

  async function begin() {
    if (!valid || submitting) return;
    setSubmitting(true); setError('');
    try { setIntent(await createFundingIntent(toMinorUnits(amount))); setStep('paypal'); }
    catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  async function confirm() {
    if (!intent || submitting) return;
    setSubmitting(true); setError(''); setStep('processing');
    try {
      const completed = await captureFunding(intent.id);
      setIntent(completed);
      setStep(completed.status === 'COMPLETED' ? 'success' : 'failed');
    } catch (cause) { setError(apiErrorMessage(cause)); setStep('failed'); }
    finally { setSubmitting(false); }
  }

  if (step === 'success') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="Your available balance has been updated." title="Money added" /></SheetScreen>;
  if (step === 'failed') return <SheetScreen footer={<ActionButton label="Try again" onPress={() => { setIntent(null); setError(''); setStep('amount'); }} />}><ResultState amount={amount} detail={error || 'PayPal could not complete this payment. Nothing was charged.'} title="Couldn’t add money" tone="failed" /></SheetScreen>;
  if (step === 'processing') return <SheetScreen><ResultState amount={amount} detail="We’re confirming your payment. This usually takes a few seconds." title="Adding your money" tone="pending" /></SheetScreen>;
  if (step === 'paypal') return <SheetScreen footer={<View style={styles.actions}><ActionButton disabled={!intent?.approvalUrl} label="Open PayPal" onPress={() => { if (intent?.approvalUrl) void Linking.openURL(intent.approvalUrl); }} /><ActionButton disabled={submitting} label={submitting ? 'Confirming…' : 'I approved in PayPal'} onPress={() => void confirm()} /></View>}><Text style={styles.title}>Approve in PayPal</Text><Text style={styles.subtitle}>Complete the payment in PayPal, then return here.</Text><View style={styles.paypal}><Text style={styles.paypalWord}>Pay<Text style={styles.pal}>Pal</Text></Text><MoneyAmount value={amount} /><Text style={styles.paypalMeta}>Paying into your ctrld balance</Text></View></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><FlowTabs active="Fund" onFund={() => undefined} onWithdraw={() => router.replace('/withdraw' as Href)} /><Text style={styles.destination}>move funds from <Text style={styles.destinationStrong}>PayPal</Text></Text><View style={styles.amountBlock}><MoneyAmount calculation={calculation} caption={`Available   $${available === null ? '—' : formatUSD(available)}`} onCalculationPress={() => { if (calculation) { setAmount(calculation); setDisplayAmount(calculation); setCalculation(null); } }} value={displayAmount} />{error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting} key={`${submitting}-${error}`} label={submitting ? 'Creating payment…' : undefined} onConfirm={() => void begin()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={setAmount} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, actions: { gap: 10 }, destination: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, destinationStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { flex: 1, justifyContent: 'center' }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 30, marginTop: -18, textAlign: 'center' }, title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6, marginTop: 24 }, subtitle: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15, lineHeight: 22, marginTop: 6 }, paypal: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.lg, marginTop: 34, padding: 24 }, paypalWord: { color: '#003087', fontFamily: fonts.black, fontSize: 30 }, pal: { color: '#009CDE' }, paypalMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13 } });
