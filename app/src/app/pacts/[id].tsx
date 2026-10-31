import { useCallback, useMemo, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, StyleSheet, Text, View } from 'react-native';
import { router, type Href, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { InfoBox } from '@/components/InfoBox';
import { PactCard } from '@/components/PactCard';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { cancelPact, getCircles, getPact, reservePactStake } from '@/api/pacts';
import { apiErrorMessage, formatUSD, getRecipients, getWallet } from '@/api/wallet';
import type { AccountabilityCircle, Pact, User, Wallet } from '@/api/types';
import { colors, fonts, radii } from '@/theme';

function shortDate(value: string) { return new Date(value).toLocaleString('en-US', { day: 'numeric', hour: 'numeric', minute: '2-digit', month: 'short' }); }

export default function PactDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [pact, setPact] = useState<Pact | null>(null); const [wallet, setWallet] = useState<Wallet | null>(null); const [circle, setCircle] = useState<AccountabilityCircle | null>(null); const [users, setUsers] = useState<User[]>([]); const [loading, setLoading] = useState(true); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
  const load = useCallback(async () => {
    if (!id) return;
    try {
      setError('');
      const [value, walletValue, circles, recipients] = await Promise.all([getPact(id), getWallet(), getCircles(), getRecipients()]);
      setPact(value); setWallet(walletValue.wallet); setCircle(circles.find((item) => item.id === value.terms.accountabilityCircleId) ?? null); setUsers(recipients.users);
    } catch (value) { setError(apiErrorMessage(value)); }
    finally { setLoading(false); }
  }, [id]);
  useFocusEffect(useCallback(() => { void load(); }, [load]));
  const people = useMemo(() => new Map(users.map((user) => [user.id, user])), [users]);
  const reserve = async () => {
    if (!pact) return; setBusy(true); setError('');
    try { const result = await reservePactStake(pact.id); setPact(result); router.replace(`/pacts/${result.id}?activated=1` as Href); }
    catch (value) { setError(apiErrorMessage(value)); setBusy(false); }
  };
  const cancel = () => pact && Alert.alert('Cancel this pact?', 'Scheduled stakes are returned to your available balance.', [{ text: 'Keep pact', style: 'cancel' }, { text: 'Cancel pact', style: 'destructive', onPress: () => { setBusy(true); void cancelPact(pact.id).then(setPact).catch((value) => setError(apiErrorMessage(value))).finally(() => setBusy(false)); } }]);
  if (loading) return <FeatureScreen scroll={false} title="Pact"><ActivityIndicator color={colors.blue} style={styles.loader} /></FeatureScreen>;
  if (!pact) return <FeatureScreen title="Pact"><View style={styles.error}><Text style={styles.errorTitle}>Couldn’t load pact</Text><Text style={styles.muted}>{error}</Text></View></FeatureScreen>;
  const available = wallet?.balances.available.amountMinor ?? 0;
  const shortage = Math.max(0, pact.terms.stake.amountMinor - available);
  const circlePeople = circle?.partnerIds.map((partner, index) => `${index + 1} ${people.get(partner)?.displayName ?? people.get(partner)?.ctrldId ?? 'Partner'}`).join(' · ') ?? 'Accountability circle';
  return <FeatureScreen title="Pact">
    <PactCard pact={pact} />
    <Text style={styles.intent}>“{pact.terms.intent}”</Text>
    {pact.status === 'AWAITING_STAKE' && shortage > 0 ? <InfoBox tone="orange">Your ${formatUSD(pact.terms.stake.amountMinor)} stake needs ${formatUSD(shortage)} more available. Add money before reserving the stake.</InfoBox> : null}
    <View style={styles.rows}>
      <Row label="Stake" value={`$${formatUSD(pact.terms.stake.amountMinor)}`} sub={pact.stakeReservationId ? 'Locked for this pact' : 'Not reserved yet'} />
      <Row label="Starts" value={shortDate(pact.terms.startsAt)} />
      <Row label="Ends" value={shortDate(pact.terms.endsAt)} />
      <Row label="Circle" value={circle?.name ?? 'Circle'} sub={circlePeople} />
      <Row label="Emergency exceptions" value={`${pact.emergencyCount} used`} />
      <Row label="If you break it" value="Stake → Commitment Reserve" />
    </View>
    {pact.progress.usage.length ? <><Text style={styles.kicker}>COUNTED TRANSACTIONS —</Text>{pact.progress.usage.map((usage) => <View key={`${usage.operationType}-${usage.operationId}`} style={styles.usage}><View style={styles.dot} /><View style={styles.usageCopy}><Text style={styles.usageTitle}>{usage.counterpartyName || (usage.destination ? `Withdraw to ${usage.destination}` : 'Wallet payment')}</Text><Text style={styles.muted}>{usage.operationType === 'WITHDRAWAL' ? 'Withdrawal' : usage.counterpartyCtrldId || 'Transfer'} · {shortDate(usage.countedAt)}</Text></View><Text style={[styles.usageAmount, usage.status === 'REVERSED' && styles.reversed]}>−${formatUSD(usage.amountMinor)}</Text></View>)}</> : null}
    <View style={styles.insights}><Text style={styles.kicker}>INSIGHTS —</Text><Text style={styles.muted}>Reserved for contextual guidance on this pact.</Text></View>
    {error ? <Text style={styles.errorText}>{error}</Text> : null}
    {pact.status === 'AWAITING_STAKE' ? <View style={styles.actions}>{shortage > 0 ? <Pressable onPress={() => router.push('/fund' as Href)} style={styles.primary}><Text style={styles.primaryText}>Add ${formatUSD(shortage)} to wallet</Text></Pressable> : <SwipeConfirm disabled={busy} label={`Swipe to reserve $${formatUSD(pact.terms.stake.amountMinor)}`} onConfirm={() => void reserve()} />}<Pressable disabled={busy} onPress={cancel}><Text style={styles.cancel}>Cancel pact</Text></Pressable></View> : null}
    {['DRAFT', 'SCHEDULED'].includes(pact.status) ? <Pressable disabled={busy} onPress={cancel}><Text style={styles.cancel}>Cancel pact</Text></Pressable> : null}
    {pact.status === 'ACTIVE' ? <Text style={styles.fixed}>Active pacts can’t be cancelled. Terms are fixed until {shortDate(pact.terms.endsAt)}.</Text> : null}
  </FeatureScreen>;
}

function Row({ label, sub, value }: { label: string; sub?: string; value: string }) { return <View style={styles.row}><Text style={styles.label}>{label}</Text><View style={styles.rowRight}><Text style={styles.value}>{value}</Text>{sub ? <Text style={styles.sub}>{sub}</Text> : null}</View></View>; }
const styles = StyleSheet.create({
  loader: { marginTop: 100 }, error: { alignItems: 'center', marginTop: 80 }, errorTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 20 }, errorText: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginVertical: 10, textAlign: 'center' }, intent: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, fontStyle: 'italic', lineHeight: 20, marginBottom: 10, paddingHorizontal: 3 },
  rows: { marginTop: 8 }, row: { alignItems: 'flex-start', borderBottomColor: colors.divider, borderBottomWidth: 1, flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 13 }, label: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13 }, rowRight: { alignItems: 'flex-end', flex: 1, marginLeft: 16 }, value: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13, textAlign: 'right' }, sub: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginTop: 3, textAlign: 'right' },
  kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginBottom: 8, marginTop: 20 }, usage: { alignItems: 'center', borderBottomColor: colors.divider, borderBottomWidth: 1, flexDirection: 'row', minHeight: 64 }, dot: { backgroundColor: colors.blue, borderRadius: 4, height: 7, marginRight: 10, width: 7 }, usageCopy: { flex: 1 }, usageTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 14 }, muted: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, lineHeight: 16 }, usageAmount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, reversed: { color: colors.textMuted, textDecorationLine: 'line-through' }, insights: { borderColor: colors.divider, borderRadius: radii.md, borderStyle: 'dashed', borderWidth: 1, marginTop: 18, padding: 13 },
  actions: { marginTop: 22 }, primary: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: radii.md, height: 56, justifyContent: 'center' }, primaryText: { color: colors.white, fontFamily: fonts.bold, fontSize: 16 }, cancel: { color: colors.redText, fontFamily: fonts.bold, fontSize: 13, marginVertical: 18, textAlign: 'center' }, fixed: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, lineHeight: 16, marginTop: 22, textAlign: 'center' },
});
