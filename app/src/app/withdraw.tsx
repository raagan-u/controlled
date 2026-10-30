import { router, type Href } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { colors, fonts } from '@/theme';
const available = 10100;
export default function WithdrawScreen() {
  const [amount, setAmount] = useState(''); const [done, setDone] = useState(false); const numeric = Number(amount); const over = numeric > available; const valid = numeric > 0 && !over;
  if (done) return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="Your withdrawal is being sent to PayPal. We’ll update the activity when it settles." title="It’s on its way" tone="pending" /></SheetScreen>;
  return <SheetScreen footer={<ActionButton disabled={!valid} label="Withdraw to PayPal" onPress={() => setDone(true)} />}><FlowTabs active="Withdraw" onFund={() => router.replace('/fund' as Href)} onWithdraw={() => undefined} /><Text style={styles.title}>Withdraw money</Text><View style={styles.availableRow}><Text style={styles.available}>AVAILABLE · $10,100.00</Text><Text onPress={() => setAmount('10100')} style={styles.max}>MAX</Text></View><MoneyAmount caption="USD" value={amount} />{over ? <View style={styles.error}><Text style={styles.errorText}>That’s more than your available balance.</Text></View> : null}<AmountKeypad onChange={setAmount} value={amount} /></SheetScreen>;
}
const styles = StyleSheet.create({ title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6, marginTop: 24 }, availableRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginTop: 10 }, available: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.2 }, max: { backgroundColor: colors.softButton, borderRadius: 8, color: colors.blue, fontFamily: fonts.bold, fontSize: 11, overflow: 'hidden', paddingHorizontal: 12, paddingVertical: 7 }, error: { backgroundColor: colors.redTint, borderRadius: 10, marginBottom: 8, padding: 10 }, errorText: { color: colors.redText, fontFamily: fonts.bold, fontSize: 13, textAlign: 'center' } });
