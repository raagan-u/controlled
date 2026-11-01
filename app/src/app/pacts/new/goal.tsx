import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { router, type Href } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { InfoBox } from '@/components/InfoBox';
import { GuidanceCard } from '@/components/GuidanceCard';
import { getPactSuggestion, type Guidance } from '@/api/ai';
import { loadLocalPactDraft, saveLocalPactDraft } from '@/api/pacts';
import { apiErrorMessage } from '@/api/wallet';
import { colors, fonts, radii } from '@/theme';

export default function PactGoalScreen() {
  const [goal, setGoal] = useState(''); const [intent, setIntent] = useState(''); const [rest, setRest] = useState<Record<string, unknown>>({}); const [guidance, setGuidance] = useState<Guidance | null>(null); const [guidanceLoading, setGuidanceLoading] = useState(false); const [guidanceError, setGuidanceError] = useState('');
  useEffect(() => { void loadLocalPactDraft().then((draft) => { if (!draft) return; setGoal(draft.goal ?? ''); setIntent(draft.intent ?? ''); const { goal: _goal, intent: _intent, updatedAt: _updatedAt, ...remaining } = draft; setRest(remaining); }); }, []);
  const persist = async () => saveLocalPactDraft({ ...rest, goal: goal.trim(), intent: intent.trim() });
  const valid = goal.trim().length > 0 && intent.trim().length > 0;
  const suggest = async () => { if (!valid) return; setGuidanceLoading(true); setGuidanceError(''); try { setGuidance(await getPactSuggestion(goal.trim(), intent.trim())); } catch (value) { setGuidanceError(apiErrorMessage(value)); } finally { setGuidanceLoading(false); } };
  const applySuggestion = async () => { if (!guidance?.suggestedTerms) return; const start = new Date(Date.now() + 3_600_000); const end = new Date(start.getTime() + guidance.suggestedTerms.durationDays * 86_400_000); const next = { ...rest, goal: goal.trim(), intent: intent.trim(), spendingLimitMinor: guidance.suggestedTerms.limitMinor, stakeMinor: guidance.suggestedTerms.stakeMinor, startsAt: start.toISOString(), endsAt: end.toISOString() }; setRest(next); await saveLocalPactDraft(next); };
  return <FeatureScreen footer={<Pressable disabled={!valid} onPress={() => void persist().then(() => router.push('/pacts/new/terms' as Href))} style={[styles.button, !valid && styles.disabled]}><Text style={styles.buttonText}>Continue</Text></Pressable>} title="New pact">
    <View style={styles.progress}><View style={styles.progressOn} /><View style={styles.progressOff} /><View style={styles.progressOff} /><View style={styles.progressOff} /></View>
    <View style={styles.stepRow}><Text style={styles.kicker}>STEP 1 OF 4 — GOAL</Text><Pressable onPress={() => void persist().then(() => router.back())}><Text style={styles.save}>Save draft</Text></Pressable></View>
    <Text style={styles.prompt}>What are you protecting, and why does it matter to you?</Text>
    <View style={styles.labelRow}><Text style={styles.label}>Pact goal</Text><Text style={styles.count}>{goal.length} / 120</Text></View>
    <TextInput maxLength={120} onChangeText={setGoal} placeholder="Keep weekends under budget" placeholderTextColor={colors.textMuted} style={styles.input} value={goal} />
    <View style={styles.labelRow}><Text style={styles.label}>Why it matters</Text><Text style={styles.count}>{intent.length} / 300</Text></View>
    <TextInput maxLength={300} multiline onChangeText={setIntent} placeholder="Fewer last-minute nights out this month" placeholderTextColor={colors.textMuted} style={[styles.input, styles.textarea]} textAlignVertical="top" value={intent} />
    <InfoBox>Your circle sees this goal and reason when you ask for an exception, so they know what you’re protecting.</InfoBox>
    {guidance || guidanceLoading ? <GuidanceCard guidance={guidance} loading={guidanceLoading} onApply={() => void applySuggestion()} /> : <Pressable disabled={!valid} onPress={() => void suggest()} style={[styles.suggest, !valid && styles.disabled]}><Text style={styles.suggestIcon}>✦</Text><View><Text style={styles.suggestTitle}>Get a grounded starting point</Text><Text style={styles.suggestion}>Uses verified wallet activity. It never moves money.</Text></View></Pressable>}
    {guidanceError ? <Text style={styles.guidanceError}>{guidanceError}</Text> : null}
  </FeatureScreen>;
}
const styles = StyleSheet.create({ progress: { flexDirection: 'row', gap: 4, marginBottom: 12 }, progressOn: { backgroundColor: colors.blue, flex: 1, height: 3 }, progressOff: { backgroundColor: colors.divider, flex: 1, height: 3 }, stepRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5 }, save: { color: colors.blue, fontFamily: fonts.bold, fontSize: 12 }, prompt: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, lineHeight: 20, marginBottom: 18, marginTop: 8 }, labelRow: { flexDirection: 'row', justifyContent: 'space-between', marginBottom: 7, marginTop: 11 }, label: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13 }, count: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11 }, input: { borderColor: colors.blue, borderRadius: radii.md, borderWidth: 1.5, color: colors.navy, fontFamily: fonts.medium, fontSize: 15, minHeight: 48, paddingHorizontal: 13, paddingVertical: 11 }, textarea: { borderColor: colors.divider, minHeight: 112 }, suggest: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.md, flexDirection: 'row', gap: 10, marginTop: 12, padding: 13 }, suggestIcon: { color: colors.blue, fontFamily: fonts.bold, fontSize: 20 }, suggestTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13 }, suggestion: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginTop: 2 }, guidanceError: { color: colors.redText, fontFamily: fonts.medium, fontSize: 10, marginTop: 7, textAlign: 'center' }, button: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: radii.md, height: 56, justifyContent: 'center' }, disabled: { opacity: 0.35 }, buttonText: { color: colors.white, fontFamily: fonts.bold, fontSize: 16 } });
