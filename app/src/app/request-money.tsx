import { router } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';

import type { User } from '@/api/types';
import { toMinorUnits } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { MoneyAmount } from '@/components/MoneyAmount';
import { RecipientPicker } from '@/components/RecipientPicker';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { TransferTabs } from '@/components/TransferTabs';
import { colors, fonts, radii } from '@/theme';

type Step = 'recipient' | 'amount' | 'pending';

export default function RequestMoneyScreen() {
  const [step, setStep] = useState<Step>('recipient');
  const [recipient, setRecipient] = useState<User | null>(null);
  const [amount, setAmount] = useState('');
  const [remark, setRemark] = useState('');
  const valid = recipient !== null && toMinorUnits(amount) > 0;

  if (step === 'pending') {
    return (
      <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}>
        <ResultState
          amount={amount}
          detail={`We’ll let you know when ${recipient?.displayName} responds.`}
          title="Request sent"
          tone="pending"
        >
          <View style={styles.pendingTag}><Text style={styles.pendingText}>PENDING</Text></View>
        </ResultState>
      </SheetScreen>
    );
  }

  if (step === 'recipient') {
    return (
      <SheetScreen>
        <TransferTabs active="Request" />
        <RecipientPicker onSelect={(user) => { setRecipient(user); setStep('amount'); }} />
      </SheetScreen>
    );
  }

  return (
    <SheetScreen>
      <View style={styles.entry}>
        <View>
          <TransferTabs active="Request" />
          <Text style={styles.requesting}>
            requesting from{' '}
            <Text onPress={() => setStep('recipient')} style={styles.requestingStrong}>
              {recipient?.displayName}⌄
            </Text>
          </Text>
        </View>
        <View style={styles.amountBlock}>
          <MoneyAmount caption={`${recipient?.displayName} can pay or decline`} value={amount} />
          <TextInput
            maxLength={80}
            onChangeText={setRemark}
            placeholder="What’s it for?"
            placeholderTextColor={colors.textMuted}
            style={styles.remark}
            value={remark}
          />
          <SwipeConfirm disabled={!valid} label="Swipe right to request" onConfirm={() => setStep('pending')} />
        </View>
        <AmountKeypad onChange={setAmount} value={amount} />
      </View>
    </SheetScreen>
  );
}

const styles = StyleSheet.create({
  entry: { flex: 1, justifyContent: 'space-between' },
  requesting: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' },
  requestingStrong: { color: colors.navy, fontFamily: fonts.bold },
  amountBlock: { flex: 1, justifyContent: 'center' },
  remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' },
  pendingTag: { backgroundColor: colors.orangeTint, borderRadius: 6, marginTop: 22, paddingHorizontal: 12, paddingVertical: 7 },
  pendingText: { color: colors.orangeText, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.2 },
});
