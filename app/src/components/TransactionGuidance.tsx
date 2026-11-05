import { Text, View, StyleSheet } from 'react-native';
import type { TransactionAssessment } from '@/api/ai';
import { colors, fonts, radii } from '@/theme';

export function TransactionGuidance({ assessment, loading, status }: { assessment: TransactionAssessment | null; loading: boolean; status: string }) {
  if (loading) return <Text style={styles.status}>Checking this payment against your pact…</Text>;
  if (!assessment) return status ? <Text style={styles.status}>{status}</Text> : null;
  const guidance = assessment.guidance;
  const fact = guidance.facts.find((item) => item.label === 'Projected at pact end') ?? guidance.facts.find((item) => item.label === 'Pact used after payment');
  return <View style={[styles.card, assessment.forecastWarning && styles.warning]}><Text style={styles.eyebrow}>{guidance.source === 'AI' ? '✦ PACT GUIDANCE' : 'SMART GUIDANCE'}{fact ? `  ·  ${fact.value}` : ''}</Text><Text numberOfLines={1} style={styles.title}>{guidance.headline}</Text><Text numberOfLines={2} style={styles.message}>{guidance.message}</Text></View>;
}

const styles = StyleSheet.create({
  card: { backgroundColor: colors.blueTint, borderRadius: radii.sm, marginHorizontal: 36, marginTop: 8, paddingHorizontal: 11, paddingVertical: 9 },
  warning: { backgroundColor: colors.orangeTint },
  eyebrow: { color: colors.blue, fontFamily: fonts.bold, fontSize: 8, letterSpacing: 0.7 },
  title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12, marginTop: 4 },
  message: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, lineHeight: 14, marginTop: 2 },
  status: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginHorizontal: 40, marginTop: 7, textAlign: 'center' },
});
