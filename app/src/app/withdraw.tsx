import { router, type Href } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { colors, fonts } from '@/theme';
const available = 10100;
export default function WithdrawScreen() {
  const [amount, setAmount] = useState(''); const [done, setDone] = useState(false); const numeric = Number(amount); const over = numeric > available; const valid = numeric > 0 && !over;
  if (done) return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="Your withdrawal is being sent to PayPal. We’ll update the activity when it settles." title="It’s on its way" tone="pending" /></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><FlowTabs active="Withdraw" onFund={() => router.replace('/fund' as Href)} onWithdraw={() => undefined} /><Text style={styles.destination}>move funds to <Text style={styles.destinationStrong}>PayPal · alex@example.com⌄</Text></Text><View style={styles.amountBlock}><MoneyAmount tone={over ? 'danger' : 'default'} value={amount} /><View style={styles.availableRow}><Text style={[styles.available, over && styles.over]}>{over ? 'More than available' : 'Available'}   $10,100.00</Text><Text onPress={() => setAmount('10100')} style={styles.max}>Max</Text></View><Text style={styles.reserve}>Staked and reserved money can’t be withdrawn.</Text><SwipeConfirm disabled={!valid} onConfirm={() => setDone(true)} /></View><AmountKeypad onChange={setAmount} value={amount} /></View></SheetScreen>;
}
const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, destination: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, destinationStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { alignItems: 'center', flex: 1, justifyContent: 'center' }, availableRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'center', marginTop: -22 }, available: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14 }, over: { color: colors.redText }, max: { backgroundColor: colors.softButton, borderRadius: 8, color: colors.blue, fontFamily: fonts.bold, fontSize: 12, marginLeft: 10, overflow: 'hidden', paddingHorizontal: 10, paddingVertical: 6 }, reserve: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 10 }, });
