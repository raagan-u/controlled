import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { router, type Href } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { InfoBox } from '@/components/InfoBox';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { clearLocalPactDraft, confirmPact, createPact, getCircles, loadLocalPactDraft, saveLocalPactDraft, type LocalPactDraft } from '@/api/pacts';
import { apiErrorMessage, formatUSD, getRecipients, getWallet } from '@/api/wallet';
import type { AccountabilityCircle, User } from '@/api/types';
import { colors, fonts } from '@/theme';

function date(value?: string) { return value ? new Date(value).toLocaleString('en-US', { day: 'numeric', hour: 'numeric', minute: '2-digit', month: 'short' }) : '—'; }
export default function ReviewPactScreen() {
  const [draft, setDraft] = useState<LocalPactDraft | null>(null); const [circle, setCircle] = useState<AccountabilityCircle | null>(null); const [users, setUsers] = useState<User[]>([]); const [available, setAvailable] = useState(0); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
  useEffect(() => { void Promise.all([loadLocalPactDraft(), getCircles(), getRecipients(), getWallet()]).then(([saved, circles, recipients, wallet]) => { setDraft(saved); setCircle(circles.find((item) => item.id === saved?.accountabilityCircleId) ?? null); setUsers(recipients.users); setAvailable(wallet.wallet.balances.available.amountMinor); }).catch((value) => setError(apiErrorMessage(value))); }, []);
  const people = useMemo(() => new Map(users.map((user) => [user.id, user])), [users]);
  const confirm = async () => {
    if (!draft?.goal || !draft.intent || !draft.spendingLimitMinor || !draft.stakeMinor || !draft.startsAt || !draft.endsAt || !draft.accountabilityCircleId) return;
    setBusy(true); setError('');
    try {
      let id = draft.serverPactId;
      if (!id) {
        const created = await createPact({ accountabilityCircleId: draft.accountabilityCircleId, breakConsequence: 'COMMITMENT_RESERVE', currency: 'USD', endsAt: draft.endsAt, goal: draft.goal, intent: draft.intent, spendingLimitMinor: draft.spendingLimitMinor, stakeMinor: draft.stakeMinor, startsAt: draft.startsAt });
        id = created.id;
        const { updatedAt: _updatedAt, ...rest } = draft;
        await saveLocalPactDraft({ ...rest, serverPactId: id });
      }
      const confirmed = await confirmPact(id);
      await clearLocalPactDraft();
      router.replace(`/pacts/${confirmed.id}` as Href);
    } catch (value) { setError(apiErrorMessage(value)); setBusy(false); }
  };
  if (!draft) return <FeatureScreen scroll={false} title="Review pact"><ActivityIndicator color={colors.blue} style={styles.loader} /></FeatureScreen>;
  const partners = circle?.partnerIds.map((id, index) => `${index + 1} ${people.get(id)?.displayName ?? 'Partner'}`).join(' · ') ?? '—';
  return <FeatureScreen title="Review pact">
    <View style={styles.progress}><View style={styles.on} /><View style={styles.on} /><View style={styles.on} /><View style={styles.on} /></View>
    <Text style={styles.kicker}>STEP 4 OF 4 — REVIEW</Text>
    <View style={styles.rows}><Row label="Goal" value={draft.goal ?? '—'} sub={draft.intent} /><Row label="Limit" value={`$${formatUSD(draft.spendingLimitMinor ?? 0)}`} /><Row label="Stake" value={`$${formatUSD(draft.stakeMinor ?? 0)}`} /><Row label="Dates" value={`${date(draft.startsAt)} — ${date(draft.endsAt)}`} /><Row label="Circle" value={circle?.name ?? '—'} sub={partners} /><Row label="If you break it" value="Stake → Commitment Reserve" /><Row label="Available after stake" value={`$${formatUSD(Math.max(0, available - (draft.stakeMinor ?? 0)))}`} /></View>
    <InfoBox>Your stake stays your money, but you can’t spend it while the pact is active. This first swipe freezes the terms. You’ll reserve the stake with a separate second swipe.</InfoBox>
    <View style={styles.check}><Text style={styles.checkIcon}>✓</Text><Text style={styles.checkText}>I understand these terms become fixed after confirmation.</Text></View>
    {error ? <Text style={styles.error}>{error}</Text> : null}
    <View style={styles.swipe}><SwipeConfirm disabled={busy} label={busy ? 'Confirming terms…' : 'Swipe to confirm terms'} onConfirm={() => void confirm()} /></View>
  </FeatureScreen>;
}
function Row({ label, sub, value }: { label: string; sub?: string; value: string }) { return <View style={styles.row}><Text style={styles.label}>{label}</Text><View style={styles.valueWrap}><Text style={styles.value}>{value}</Text>{sub ? <Text style={styles.sub}>{sub}</Text> : null}</View></View>; }
const styles = StyleSheet.create({ loader: { marginTop: 100 }, progress: { flexDirection: 'row', gap: 4, marginBottom: 12 }, on: { backgroundColor: colors.blue, flex: 1, height: 3 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 }, rows: { marginTop: 8 }, row: { alignItems: 'flex-start', borderBottomColor: colors.divider, borderBottomWidth: 1, flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 14 }, label: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13 }, valueWrap: { alignItems: 'flex-end', flex: 1, marginLeft: 16 }, value: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13, textAlign: 'right' }, sub: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginTop: 3, textAlign: 'right' }, check: { alignItems: 'flex-start', flexDirection: 'row', gap: 9, marginTop: 8 }, checkIcon: { backgroundColor: colors.blue, borderRadius: 5, color: colors.white, fontFamily: fonts.bold, fontSize: 11, overflow: 'hidden', paddingHorizontal: 5, paddingVertical: 3 }, checkText: { color: colors.navy, flex: 1, fontFamily: fonts.bold, fontSize: 12, lineHeight: 17 }, swipe: { marginTop: 38 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginTop: 12, textAlign: 'center' } });
