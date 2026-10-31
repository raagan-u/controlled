import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withDelay, withTiming } from 'react-native-reanimated';
import { PressableScale, rise } from '@/motion';
import type { Pact } from '@/api/types';
import { colors, fonts, radii } from '@/theme';
import { formatUSD } from '@/api/wallet';

type Tone = 'blue' | 'green' | 'orange' | 'neutral' | 'red';
const toneColors: Record<Tone, { accent: string; background: string }> = {
  blue: { accent: colors.blue, background: colors.blueTint },
  green: { accent: colors.greenText, background: colors.greenTint },
  orange: { accent: colors.orange, background: colors.orangeTint },
  red: { accent: colors.redText, background: colors.redTint },
  neutral: { accent: colors.textMuted, background: colors.surface },
};

export function pactTone(pact: Pact): Tone {
  if (pact.progressAvailable === false) return 'neutral';
  if (pact.status === 'CANCELLED' || pact.status === 'DRAFT' || pact.status === 'AWAITING_STAKE') return 'neutral';
  if (pact.outcome === 'BROKEN' || pact.progress.overageMinor > 0) return 'red';
  if (pact.status === 'RESOLVED' || pact.progress.usedMinor / pact.terms.spendingLimit.amountMinor < 0.5) return 'green';
  if (pact.progress.usedMinor / pact.terms.spendingLimit.amountMinor >= 0.8) return 'orange';
  return 'blue';
}

export function PactCard({ compact, onPress, pact }: { compact?: boolean; onPress?: () => void; pact: Pact }) {
  const [now] = useState(Date.now);
  const tone = toneColors[pactTone(pact)];
  const limit = pact.terms.spendingLimit.amountMinor;
  const progressAvailable = pact.progressAvailable !== false;
  const progress = Math.min(1, limit ? pact.progress.usedMinor / limit : 0);
  const fill = useSharedValue(0);
  const target = Math.max(3, progress * 100);
  useEffect(() => { fill.set(withDelay(250, withTiming(target, { duration: 700, easing: Easing.out(Easing.cubic) }))); }, [fill, target]);
  const fillStyle = useAnimatedStyle(() => ({ width: `${fill.value}%` }));
  const days = Math.max(0, Math.ceil((new Date(pact.terms.endsAt).getTime() - now) / 86_400_000));
  return (
    <PressableScale disabled={!onPress} entering={rise()} onPress={onPress} scaleTo={0.98} style={[styles.card, compact && styles.compact, { backgroundColor: tone.background }]}>
      <View style={styles.top}><Text numberOfLines={2} style={styles.goal}>{pact.terms.goal}</Text><View style={[styles.badge, { backgroundColor: colors.white }]}><Text style={[styles.badgeText, { color: tone.accent }]}>{pact.status.replaceAll('_', ' ')}</Text></View></View>
      {progressAvailable ? <><View style={styles.amountRow}><Text style={styles.amount}>${formatUSD(pact.progress.usedMinor)}</Text><Text style={styles.of}> spent of ${formatUSD(limit)}</Text><Text style={styles.left}>${formatUSD(pact.progress.remainingMinor)} left</Text></View><View style={styles.track}><Animated.View style={[styles.fill, { backgroundColor: tone.accent }, fillStyle]} /></View></> : <View style={styles.unavailable}><Text style={styles.unavailableTitle}>Progress unavailable</Text><Text style={styles.of}>Reconnect to the updated API to refresh spending.</Text></View>}
      <Text style={styles.meta}>{pact.status === 'RESOLVED' ? 'Completed' : `${days} day${days === 1 ? '' : 's'} left`} · {pact.emergencyCount} emergenc{pact.emergencyCount === 1 ? 'y' : 'ies'}</Text>
    </PressableScale>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: radii.lg, marginBottom: 12, padding: 16 }, compact: { marginBottom: 8, padding: 13 },
  top: { alignItems: 'flex-start', flexDirection: 'row', gap: 8, justifyContent: 'space-between' },
  goal: { color: colors.navy, flex: 1, fontFamily: fonts.bold, fontSize: 16, lineHeight: 21 },
  badge: { borderRadius: radii.tag, paddingHorizontal: 8, paddingVertical: 5 },
  badgeText: { fontFamily: fonts.bold, fontSize: 9, letterSpacing: 0.4 },
  amountRow: { alignItems: 'baseline', flexDirection: 'row', marginTop: 12 },
  amount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 20 },
  of: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12 },
  left: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12, marginLeft: 'auto' },
  track: { backgroundColor: 'rgba(90,100,120,0.16)', borderRadius: 4, height: 6, marginTop: 9, overflow: 'hidden' },
  fill: { borderRadius: 4, height: 6 },
  meta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, marginTop: 9 },
  unavailable: { marginTop: 12 }, unavailableTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13, marginBottom: 3 },
});
