import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import type { Guidance } from '@/api/ai';
import { formatUSD } from '@/api/wallet';
import { colors, fonts, radii } from '@/theme';

type Props = {
  guidance?: Guidance | null;
  loading?: boolean;
  onApply?: () => void;
};

const palettes = {
  NEUTRAL: { accent: colors.blue, background: colors.blueTint },
  POSITIVE: { accent: colors.greenText, background: colors.greenTint },
  CAUTION: { accent: colors.orangeText, background: colors.orangeTint },
  CRITICAL: { accent: colors.redText, background: colors.redTint },
} as const;

export function GuidanceCard({ guidance, loading, onApply }: Props) {
  if (loading) return <View style={styles.loading}><ActivityIndicator color={colors.blue} size="small" /><Text style={styles.loadingText}>Looking at the verified facts…</Text></View>;
  if (!guidance) return null;
  const palette = palettes[guidance.tone];
  return <View style={[styles.card, { backgroundColor: palette.background }]}>
    <View style={styles.heading}><Text style={[styles.eyebrow, { color: palette.accent }]}>{guidance.source === 'AI' ? '✦ AI GUIDANCE' : 'SMART GUIDANCE'}</Text><Text style={styles.verified}>VERIFIED FACTS</Text></View>
    <Text style={styles.title}>{guidance.headline}</Text>
    <Text style={styles.message}>{guidance.message}</Text>
    {guidance.facts.length ? <View style={styles.facts}>{guidance.facts.map((fact) => <View key={fact.label} style={styles.fact}><Text style={styles.factLabel}>{fact.label}</Text><Text style={styles.factValue}>{fact.value}</Text></View>)}</View> : null}
    {guidance.suggestedTerms ? <View style={styles.terms}><Term label="Limit" value={`$${formatUSD(guidance.suggestedTerms.limitMinor)}`} /><Term label="Stake" value={`$${formatUSD(guidance.suggestedTerms.stakeMinor)}`} /><Term label="Duration" value={`${guidance.suggestedTerms.durationDays} days`} /></View> : null}
    {onApply && guidance.suggestedTerms ? <Pressable accessibilityRole="button" onPress={onApply} style={[styles.apply, { backgroundColor: palette.accent }]}><Text style={styles.applyText}>Use these terms</Text></Pressable> : null}
  </View>;
}

function Term({ label, value }: { label: string; value: string }) {
  return <View style={styles.term}><Text style={styles.factLabel}>{label}</Text><Text style={styles.termValue}>{value}</Text></View>;
}

const styles = StyleSheet.create({
  card: { borderRadius: radii.lg, marginTop: 16, padding: 15 },
  heading: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' },
  eyebrow: { fontFamily: fonts.bold, fontSize: 9, letterSpacing: 1.2 },
  verified: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 8, letterSpacing: 1 },
  title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 17, marginTop: 10 },
  message: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, lineHeight: 18, marginTop: 4 },
  facts: { flexDirection: 'row', gap: 6, marginTop: 13 },
  fact: { backgroundColor: 'rgba(255,255,255,0.72)', borderRadius: radii.sm, flex: 1, minHeight: 48, paddingHorizontal: 8, paddingVertical: 7 },
  factLabel: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 8 },
  factValue: { color: colors.navy, fontFamily: fonts.bold, fontSize: 11, marginTop: 3 },
  terms: { backgroundColor: 'rgba(255,255,255,0.72)', borderRadius: radii.md, flexDirection: 'row', marginTop: 8, padding: 10 },
  term: { flex: 1 },
  termValue: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13, marginTop: 3 },
  apply: { alignItems: 'center', borderRadius: radii.md, height: 44, justifyContent: 'center', marginTop: 10 },
  applyText: { color: colors.white, fontFamily: fonts.bold, fontSize: 13 },
  loading: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.md, flexDirection: 'row', gap: 9, marginTop: 16, padding: 14 },
  loadingText: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11 },
});
