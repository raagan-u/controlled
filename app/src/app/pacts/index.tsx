import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { router, type Href } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { PactCard } from '@/components/PactCard';
import { getPacts, loadLocalPactDraft } from '@/api/pacts';
import type { LocalPactDraft } from '@/api/pacts';
import type { Pact } from '@/api/types';
import { colors, fonts, radii } from '@/theme';
import { useActiveRefresh } from '@/hooks/useActiveRefresh';

export default function PactsScreen() {
  const [pacts, setPacts] = useState<Pact[]>([]);
  const [localDraft, setLocalDraft] = useState<LocalPactDraft | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    try {
      setError('');
      const [items, draft] = await Promise.all([getPacts(), loadLocalPactDraft()]);
      setPacts(items); setLocalDraft(draft);
    } catch (value) { setError(value instanceof Error ? value.message : 'Could not load pacts.'); }
    finally { setLoading(false); }
  }, []);
  useActiveRefresh(load);
  const current = pacts.filter((pact) => ['ACTIVE', 'SCHEDULED', 'AWAITING_STAKE'].includes(pact.status));
  const drafts = pacts.filter((pact) => pact.status === 'DRAFT');
  return (
    <FeatureScreen title="Pacts">
      {loading ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : null}
      {error ? <Pressable onPress={() => void load()} style={styles.error}><Text style={styles.errorTitle}>Couldn’t load pacts</Text><Text style={styles.copy}>{error} Tap to retry.</Text></Pressable> : null}
      {!loading && !error && current.length === 0 && drafts.length === 0 && !localDraft ? <View style={styles.empty}><View style={styles.shield}><Text style={styles.shieldIcon}>◇</Text></View><Text style={styles.emptyTitle}>Create your first pact</Text><Text style={styles.emptyCopy}>Pick a goal, set a spending limit, back it with a stake, and choose people who’ll keep you honest.</Text></View> : null}
      {current.length ? <><Text style={styles.kicker}>CURRENT —</Text>{current.map((pact) => <PactCard key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} pact={pact} />)}</> : null}
      {localDraft || drafts.length ? <><Text style={styles.kicker}>DRAFTS —</Text>{localDraft ? <Pressable onPress={() => router.push('/pacts/new/goal' as Href)} style={styles.draft}><View><Text style={styles.draftTitle}>{localDraft.goal || 'New pact'}</Text><Text style={styles.copy}>Saved on this device</Text></View><Text style={styles.continue}>Continue</Text></Pressable> : null}{drafts.map((pact) => <PactCard compact key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} pact={pact} />)}</> : null}
      <Pressable onPress={() => router.push('/pacts/recovery' as Href)} style={styles.recovery}><View style={styles.recoveryIcon}><Text style={styles.recoveryIconText}>↗</Text></View><View style={styles.recoveryCopy}><Text style={styles.recoveryTitle}>Stakes & recovery</Text><Text style={styles.copy}>See what’s staked and how Commitment Reserve unlocks.</Text></View><Text style={styles.arrow}>→</Text></Pressable>
      <Pressable onPress={() => router.push('/pacts/history' as Href)} style={styles.history}><Text style={styles.historyText}>Pact history</Text><Text style={styles.arrow}>→</Text></Pressable>
      <Pressable onPress={() => router.push('/pacts/new/goal' as Href)} style={styles.create}><Text style={styles.plus}>＋</Text><Text style={styles.createText}>Create pact</Text></Pressable>
    </FeatureScreen>
  );
}

const styles = StyleSheet.create({
  create: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: radii.md, flexDirection: 'row', height: 52, justifyContent: 'center', marginBottom: 8, marginTop: 6 }, plus: { color: colors.white, fontFamily: fonts.medium, fontSize: 22, marginRight: 5 }, createText: { color: colors.white, fontFamily: fonts.bold, fontSize: 15 },
  loader: { marginTop: 60 }, error: { alignItems: 'center', backgroundColor: colors.redTint, borderRadius: radii.lg, marginTop: 18, padding: 20 }, errorTitle: { color: colors.redText, fontFamily: fonts.bold, fontSize: 16 }, copy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 4 },
  empty: { alignItems: 'center', flex: 1, justifyContent: 'center', minHeight: 500, paddingHorizontal: 28 }, shield: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: 40, height: 80, justifyContent: 'center', width: 80 }, shieldIcon: { color: colors.blue, fontFamily: fonts.bold, fontSize: 36 }, emptyTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25, marginTop: 22 }, emptyCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, lineHeight: 21, marginTop: 10, textAlign: 'center' },
  kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.6, marginBottom: 10, marginTop: 3 }, draft: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.lg, flexDirection: 'row', justifyContent: 'space-between', marginBottom: 10, padding: 16 }, draftTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, continue: { color: colors.blue, fontFamily: fonts.bold, fontSize: 13 },
  history: { alignItems: 'center', flexDirection: 'row', justifyContent: 'center', marginTop: 18, padding: 14 }, historyText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14 }, arrow: { color: colors.blue, fontFamily: fonts.bold, fontSize: 18, marginLeft: 8 },
	recovery: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: radii.lg, flexDirection: 'row', marginTop: 16, padding: 15 }, recoveryIcon: { alignItems: 'center', backgroundColor: colors.white, borderRadius: 14, height: 42, justifyContent: 'center', width: 42 }, recoveryIconText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 20 }, recoveryCopy: { flex: 1, marginLeft: 12 }, recoveryTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16 },
});
