import { router, useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput } from 'react-native';
import { AmountKeypad } from '@/components/AmountKeypad';
import { ContactRow } from '@/components/ContactRow';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { contacts } from '@/data/demo';
import { colors, fonts, radii } from '@/theme';
type Step = 'recipient' | 'amount' | 'success';
export default function SendScreen() {
  const params = useLocalSearchParams<{ recipient?: string }>(); const preset = typeof params.recipient === 'string' ? params.recipient : '';
  const [step, setStep] = useState<Step>(preset ? 'amount' : 'recipient'); const [recipient, setRecipient] = useState(preset); const [search, setSearch] = useState(''); const [amount, setAmount] = useState(''); const [remark, setRemark] = useState('');
  const filtered = useMemo(() => contacts.filter((person) => `${person.name} ${person.username}`.toLowerCase().includes(search.toLowerCase())), [search]);
  if (step === 'success') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Done</Text>}><ResultState amount={amount} detail={`Sent to ${recipient}${remark ? ` · ${remark}` : ''}`} title="Money sent" /></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><Text style={styles.title}>Who are you sending to?</Text><TextInput autoCapitalize="none" onChangeText={setSearch} placeholder="Search name or username" placeholderTextColor={colors.textMuted} style={styles.search} value={search} /><Text style={styles.kicker}>RECENTS</Text><ScrollView showsVerticalScrollIndicator={false}>{filtered.map((person) => <ContactRow {...person} key={person.username} onPress={() => { setRecipient(person.name); setStep('amount'); }} />)}</ScrollView></SheetScreen>;
  return <SheetScreen footer={<SwipeConfirm disabled={Number(amount) <= 0} label="Swipe right to send" onConfirm={() => setStep('success')} />}><Text style={styles.to}>TO · {recipient.toUpperCase()}</Text><MoneyAmount caption="AVAILABLE · $10,100.00" value={amount} /><TextInput maxLength={80} onChangeText={setRemark} placeholder="Add a remark (optional)" placeholderTextColor={colors.textMuted} style={styles.remark} value={remark} /><AmountKeypad onChange={setAmount} value={amount} /></SheetScreen>;
}
const styles = StyleSheet.create({ title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6 }, search: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 15, height: 48, marginTop: 20, paddingHorizontal: 16 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.5, marginTop: 24 }, to: { color: colors.blue, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.5, textAlign: 'center' }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginBottom: 12, paddingHorizontal: 15 }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' } });
