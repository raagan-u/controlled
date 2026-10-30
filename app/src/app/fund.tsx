import { router, type Href } from 'expo-router';
import { useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { colors, fonts, radii } from '@/theme';
type Step = 'amount' | 'paypal' | 'processing' | 'success' | 'failed';
export default function FundScreen() {
  const [amount, setAmount] = useState(''); const [step, setStep] = useState<Step>('amount'); const valid = Number(amount) > 0;
  if (step === 'success') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="Your available balance has been updated." title="Money added" /></SheetScreen>;
  if (step === 'failed') return <SheetScreen footer={<ActionButton label="Try again" onPress={() => setStep('paypal')} />}><ResultState amount={amount} detail="PayPal could not complete this payment. Nothing was charged." title="Couldn’t add money" tone="failed" /></SheetScreen>;
  if (step === 'processing') return <SheetScreen footer={<View style={styles.debugRow}><Pressable onPress={() => setStep('failed')}><Text style={styles.failLink}>Payment failed</Text></Pressable><ActionButton label="Payment completed" onPress={() => setStep('success')} /></View>}><ResultState amount={amount} detail="We’re confirming your payment with PayPal. This usually takes a few seconds." title="Adding your money" tone="pending" /></SheetScreen>;
  if (step === 'paypal') return <SheetScreen footer={<ActionButton label="I approved in PayPal" onPress={() => setStep('processing')} />}><Text style={styles.title}>Approve in PayPal</Text><Text style={styles.subtitle}>Complete the payment in PayPal, then return here.</Text><View style={styles.paypal}><Text style={styles.paypalWord}>Pay<Text style={styles.pal}>Pal</Text></Text><MoneyAmount value={amount} /><Text style={styles.paypalMeta}>Paying into your ctrld balance</Text></View></SheetScreen>;
  return <SheetScreen footer={<ActionButton disabled={!valid} label="Continue with PayPal" onPress={() => setStep('paypal')} />}><FlowTabs active="Fund" onFund={() => undefined} onWithdraw={() => router.replace('/withdraw' as Href)} /><Text style={styles.title}>Fund your balance</Text><Text style={styles.subtitle}>Add money securely through PayPal.</Text><MoneyAmount caption="USD" value={amount} /><AmountKeypad onChange={setAmount} value={amount} /></SheetScreen>;
}
const styles = StyleSheet.create({ title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6, marginTop: 24 }, subtitle: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15, lineHeight: 22, marginTop: 6 }, paypal: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.lg, marginTop: 34, padding: 24 }, paypalWord: { color: '#003087', fontFamily: fonts.black, fontSize: 30 }, pal: { color: '#009CDE' }, paypalMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13 }, debugRow: { gap: 10 }, failLink: { color: colors.redText, fontFamily: fonts.bold, fontSize: 13, padding: 8, textAlign: 'center' } });
