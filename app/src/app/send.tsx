import { router, useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import { AmountKeypad } from '@/components/AmountKeypad';
import { ContactRow } from '@/components/ContactRow';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { TransferTabs } from '@/components/TransferTabs';
import { contacts } from '@/data/demo';
import { colors, fonts, radii } from '@/theme';
type Step = 'recipient' | 'amount' | 'success';
export default function SendScreen() {
  const params = useLocalSearchParams<{ recipient?: string }>(); const preset = typeof params.recipient === 'string' ? params.recipient : '';
  const [step, setStep] = useState<Step>(preset ? 'amount' : 'recipient'); const [recipient, setRecipient] = useState(preset); const [search, setSearch] = useState(''); const [amount, setAmount] = useState(''); const [remark, setRemark] = useState('');
  const filtered = useMemo(() => contacts.filter((person) => `${person.name} ${person.username}`.toLowerCase().includes(search.toLowerCase())), [search]);
  if (step === 'success') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Done</Text>}><ResultState amount={amount} detail={`Sent to ${recipient}${remark ? ` · ${remark}` : ''}`} title="Money sent" /></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><TransferTabs active="Send" /><TextInput autoCapitalize="none" onChangeText={setSearch} placeholder="Name or email" placeholderTextColor={colors.textMuted} style={styles.search} value={search} /><Text style={styles.kicker}>RECENTS —</Text><ScrollView showsVerticalScrollIndicator={false}>{filtered.map((person) => <ContactRow {...person} key={person.username} onPress={() => { setRecipient(person.name); setStep('amount'); }} />)}<Text style={styles.kicker}>NOT ON CTRLD —</Text><ContactRow initials="＋" name="Invite by email" onPress={() => undefined} username="They need an account to receive money" /></ScrollView></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><View><TransferTabs active="Send" /><Text style={styles.payee}>paying to <Text style={styles.payeeStrong}>{recipient}⌄</Text></Text></View><View style={styles.amountBlock}><MoneyAmount caption="Available $1,248.60" value={amount} /><TextInput maxLength={80} onChangeText={setRemark} placeholder="Add a remark" placeholderTextColor={colors.textMuted} style={styles.remark} value={remark} /><SwipeConfirm disabled={Number(amount) <= 0} onConfirm={() => setStep('success')} /></View><AmountKeypad onChange={setAmount} value={amount} /></View></SheetScreen>;
}
const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, payee: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, payeeStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { flex: 1, justifyContent: 'center' }, search: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 15, height: 48, marginTop: 16, paddingHorizontal: 16 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginTop: 22 }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' } });
