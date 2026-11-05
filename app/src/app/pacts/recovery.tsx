import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { router, type Href } from 'expo-router';
import { getStakeRecovery } from '@/api/pacts';
import type { StakeRecovery } from '@/api/types';
import { formatUSD } from '@/api/wallet';
import { FeatureScreen } from '@/components/FeatureScreen';
import { useActiveRefresh } from '@/hooks/useActiveRefresh';
import { colors, fonts, radii } from '@/theme';

const shortDate = (value: string) => new Date(value).toLocaleDateString('en-US', { day: 'numeric', month: 'short' });

export default function StakeRecoveryScreen() {
  const [data, setData] = useState<StakeRecovery | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    try {
      setData(await getStakeRecovery());
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not load stake recovery.');
    } finally {
      setLoading(false);
    }
  }, []);
  useActiveRefresh(load);

  const total = data?.totalReservedMinor ?? 0;
  const recovered = data?.recoveredMinor ?? 0;
  const progress = total > 0 ? Math.min(100, (recovered / total) * 100) : 0;

  return <FeatureScreen title="Stakes & recovery">
    {loading && !data ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : null}
    {error && !data ? <Pressable onPress={() => void load()} style={styles.error}><Text style={styles.errorTitle}>Couldn’t load stakes</Text><Text style={styles.errorCopy}>{error} Tap to retry.</Text></Pressable> : null}
    {data ? <>
      <Text style={styles.kicker}>STAKE SUMMARY —</Text>
      <View style={styles.summaryCard}>
        <View style={styles.summaryRow}><View style={[styles.summaryMark, styles.stakedMark]}><Text style={styles.summaryMarkText}>◆</Text></View><View style={styles.summaryCopy}><Text style={styles.summaryLabel}>Currently staked</Text><Text style={styles.summaryMeta}>{data.stakedPactCount} {data.stakedPactCount === 1 ? 'active pact' : 'active pacts'}</Text></View><Text style={styles.summaryAmount}>${formatUSD(data.stakedMinor)}</Text></View>
        <View style={styles.divider} />
        <View style={styles.summaryRow}><View style={[styles.summaryMark, styles.reserveMark]}><Text style={styles.reserveMarkText}>↗</Text></View><View style={styles.summaryCopy}><Text style={styles.summaryLabel}>Commitment Reserve</Text><Text style={styles.summaryMeta}>Still yours · temporarily locked</Text></View><Text style={styles.summaryAmount}>${formatUSD(data.commitmentReserveMinor)}</Text></View>
      </View>

      <View style={styles.planCard}>
        <Text style={styles.planKicker}>RECOVERY PLAN</Text>
        {total > 0 ? <>
          <Text style={styles.planAmount}>${formatUSD(recovered)} <Text style={styles.planOf}>of ${formatUSD(total)} recovered</Text></Text>
          <View style={styles.track}><View style={[styles.fill, { width: `${progress}%` }]} /></View>
          <View style={styles.planLine}><Text style={styles.planLineLabel}>Still to unlock</Text><Text style={styles.planLineValue}>${formatUSD(data.commitmentReserveMinor)} in kept-pact stakes</Text></View>
          <View style={styles.projected}><View><Text style={styles.projectedLabel}>ON TRACK</Text><Text style={styles.projectedCopy}>From eligible active pacts</Text></View><Text style={styles.projectedAmount}>+${formatUSD(data.projectedRecoveryMinor)}</Text></View>
        </> : <View style={styles.clearState}><Text style={styles.clearTitle}>No stake is locked</Text><Text style={styles.clearCopy}>You don’t currently have anything in Commitment Reserve.</Text></View>}
      </View>

      {data.pacts.length ? <><Text style={styles.sectionTitle}>RECOVERY PACTS —</Text>{data.pacts.map((pact) => <Pressable key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} style={[styles.pactCard, !pact.recoveryEligible && styles.pactCardIneligible]}><View style={styles.pactTop}><View style={styles.pactCopy}><Text numberOfLines={2} style={styles.pactGoal}>{pact.goal}</Text><Text style={styles.pactMeta}>{pact.status === 'SCHEDULED' ? 'Scheduled' : 'Active'} · ends {shortDate(pact.endsAt)}</Text></View><Text style={styles.pactStake}>${formatUSD(pact.stakeMinor)} stake</Text></View><View style={[styles.unlockRow, !pact.recoveryEligible && styles.unlockRowIneligible]}><Text style={pact.recoveryEligible ? styles.unlockLabel : styles.ineligibleLabel}>{pact.recoveryEligible ? 'If kept' : 'Too short to qualify'}</Text><Text style={pact.recoveryEligible ? styles.unlockAmount : styles.ineligibleAmount}>{pact.recoveryEligible ? `Unlocks $${formatUSD(pact.projectedRecoveryMinor)}` : 'Needs 7+ days'}</Text></View></Pressable>)}</> : null}

      <Pressable onPress={() => router.push('/pacts/new/goal' as Href)} style={styles.create}><Text style={styles.createPlus}>＋</Text><Text style={styles.createText}>Create recovery pact</Text></Pressable>
      <View style={styles.note}><Text style={styles.noteIcon}>i</Text><Text style={styles.noteText}>Only pacts lasting at least 7 days can unlock Commitment Reserve when kept. Broken, cancelled, or shorter pacts recover nothing.</Text></View>
    </> : null}
  </FeatureScreen>;
}

const styles = StyleSheet.create({
  loader: { marginTop: 80 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.7, marginBottom: 10, marginTop: 2 },
  error: { alignItems: 'center', backgroundColor: colors.redTint, borderRadius: radii.lg, marginTop: 40, padding: 20 }, errorTitle: { color: colors.redText, fontFamily: fonts.bold, fontSize: 17 }, errorCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 5, textAlign: 'center' },
  summaryCard: { backgroundColor: colors.surface, borderRadius: radii.lg, paddingHorizontal: 16 }, summaryRow: { alignItems: 'center', flexDirection: 'row', minHeight: 82 }, summaryMark: { alignItems: 'center', borderRadius: 14, height: 44, justifyContent: 'center', marginRight: 12, width: 44 }, stakedMark: { backgroundColor: colors.blueTint }, reserveMark: { backgroundColor: colors.orangeTint }, summaryMarkText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 18 }, reserveMarkText: { color: colors.orange, fontFamily: fonts.bold, fontSize: 20 }, summaryCopy: { flex: 1 }, summaryLabel: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, summaryMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 3 }, summaryAmount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 21, letterSpacing: -0.4 }, divider: { backgroundColor: colors.divider, height: 1, marginLeft: 56 },
  planCard: { backgroundColor: colors.navy, borderRadius: radii.lg, marginTop: 14, padding: 18 }, planKicker: { color: colors.navyMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.6 }, planAmount: { color: colors.white, fontFamily: fonts.bold, fontSize: 27, letterSpacing: -0.6, marginTop: 12 }, planOf: { color: colors.navyMuted2, fontFamily: fonts.medium, fontSize: 14, letterSpacing: 0 }, track: { backgroundColor: '#354054', borderRadius: 5, height: 8, marginTop: 12, overflow: 'hidden' }, fill: { backgroundColor: colors.green, borderRadius: 5, height: 8 }, planLine: { flexDirection: 'row', justifyContent: 'space-between', marginTop: 10 }, planLineLabel: { color: colors.navyMuted2, fontFamily: fonts.medium, fontSize: 12 }, planLineValue: { color: colors.white, fontFamily: fonts.bold, fontSize: 12 }, projected: { alignItems: 'center', backgroundColor: '#243149', borderRadius: radii.md, flexDirection: 'row', justifyContent: 'space-between', marginTop: 16, padding: 14 }, projectedLabel: { color: colors.green, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 }, projectedCopy: { color: colors.navyMuted2, fontFamily: fonts.medium, fontSize: 11, marginTop: 3 }, projectedAmount: { color: colors.green, fontFamily: fonts.bold, fontSize: 23 }, clearState: { paddingVertical: 22 }, clearTitle: { color: colors.white, fontFamily: fonts.bold, fontSize: 22 }, clearCopy: { color: colors.navyMuted2, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginTop: 6 },
  sectionTitle: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.7, marginBottom: 10, marginTop: 22 }, pactCard: { backgroundColor: colors.greenTint, borderRadius: radii.lg, marginBottom: 10, padding: 16 }, pactCardIneligible: { backgroundColor: colors.orangeTint }, pactTop: { alignItems: 'flex-start', flexDirection: 'row' }, pactCopy: { flex: 1, paddingRight: 10 }, pactGoal: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16, lineHeight: 21 }, pactMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 4 }, pactStake: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13 }, unlockRow: { borderTopColor: 'rgba(14,122,85,0.14)', borderTopWidth: 1, flexDirection: 'row', justifyContent: 'space-between', marginTop: 13, paddingTop: 11 }, unlockRowIneligible: { borderTopColor: 'rgba(138,74,6,0.16)' }, unlockLabel: { color: colors.greenText, fontFamily: fonts.medium, fontSize: 12 }, unlockAmount: { color: colors.greenText, fontFamily: fonts.bold, fontSize: 13 }, ineligibleLabel: { color: colors.orangeText, fontFamily: fonts.medium, fontSize: 12 }, ineligibleAmount: { color: colors.orangeText, fontFamily: fonts.bold, fontSize: 13 },
  create: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: radii.md, flexDirection: 'row', height: 54, justifyContent: 'center', marginTop: 12 }, createPlus: { color: colors.white, fontFamily: fonts.medium, fontSize: 22, marginRight: 6 }, createText: { color: colors.white, fontFamily: fonts.bold, fontSize: 16 }, note: { alignItems: 'flex-start', backgroundColor: colors.blueTint, borderRadius: radii.md, flexDirection: 'row', marginBottom: 4, marginTop: 12, padding: 13 }, noteIcon: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14, marginRight: 9 }, noteText: { color: colors.textMuted, flex: 1, fontFamily: fonts.medium, fontSize: 12, lineHeight: 17 },
});
