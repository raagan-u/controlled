import { useEffect, useRef, useState } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { TransactionAssessment } from '@/api/ai';
import { formatUSD } from '@/api/wallet';
import { colors, fonts, radii } from '@/theme';
import { ActionButton } from './ActionButton';

const relationshipLabels: Record<TransactionAssessment['relationship'], string> = {
  ALIGNED: 'Potentially aligned with your pact',
  CONFLICTING: 'May conflict with your pact',
  EXCEPTIONAL: 'Looks like an exceptional expense',
  UNCLEAR: 'Relationship to your pact is unclear',
};

const changeLabels: Record<TransactionAssessment['pattern']['change'], string> = {
  DECREASING: 'Spending is decreasing',
  INCREASING: 'Spending is increasing',
  NEW_ACTIVITY: 'A new spending pattern',
  STABLE: 'Spending is mostly stable',
};

export function TransactionGuidance({ assessment, loading, status }: { assessment: TransactionAssessment | null; loading: boolean; status: string }) {
  const [visible, setVisible] = useState(false);
  const lastOpenedAssessment = useRef('');
  const assessmentKey = assessment ? assessment.id ?? `${assessment.guidance.generatedAt}:${assessment.guidance.headline}` : '';

  useEffect(() => {
    if (!assessment || assessmentKey === lastOpenedAssessment.current) return;
    lastOpenedAssessment.current = assessmentKey;
    setVisible(true);
  }, [assessment, assessmentKey]);

  if (loading) return <Text style={styles.status}>Checking this payment against your pact…</Text>;
  if (!assessment) return status ? <Text style={styles.status}>{status}</Text> : null;

  const guidance = assessment.guidance;
  const pattern = assessment.pattern;
  const close = () => setVisible(false);

  return (
    <>
      <Pressable
        accessibilityHint="Opens the full pact guidance"
        accessibilityRole="button"
        onPress={() => setVisible(true)}
        style={({ pressed }) => [styles.trigger, assessment.forecastWarning && styles.warning, pressed && styles.pressed]}
      >
        <View style={styles.triggerCopy}>
          <Text style={styles.eyebrow}>{guidance.source === 'AI' ? '✦ PACT GUIDANCE' : 'SMART GUIDANCE'}</Text>
          <Text style={styles.triggerTitle}>{assessment.forecastWarning ? 'Review before continuing' : 'Guidance ready'}</Text>
        </View>
        <Text style={styles.viewLabel}>VIEW</Text>
      </Pressable>

      <Modal animationType="fade" onRequestClose={close} statusBarTranslucent transparent visible={visible}>
        <View style={styles.modalRoot}>
          <Pressable accessibilityLabel="Close guidance" onPress={close} style={styles.scrim} />
          <View style={styles.sheet}>
            <View style={styles.handle} />
            <View style={styles.headingRow}>
              <Text style={styles.modalEyebrow}>{guidance.source === 'AI' ? '✦ PACT GUIDANCE' : 'SMART GUIDANCE'}</Text>
              <Pressable accessibilityLabel="Close guidance" accessibilityRole="button" hitSlop={12} onPress={close}>
                <Text style={styles.close}>×</Text>
              </Pressable>
            </View>

            <ScrollView contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
              <Text style={styles.title}>{guidance.headline}</Text>
              <Text style={styles.message}>{guidance.message}</Text>

              <View style={styles.relationshipBadge}>
                <Text style={styles.relationshipText}>{relationshipLabels[assessment.relationship]}</Text>
              </View>

              {guidance.facts.length > 0 && (
                <View style={styles.section}>
                  <Text style={styles.sectionTitle}>WHAT WE CHECKED</Text>
                  {guidance.facts.map((fact) => (
                    <View key={`${fact.label}:${fact.value}`} style={styles.factRow}>
                      <Text style={styles.factLabel}>{fact.label}</Text>
                      <Text style={styles.factValue}>{fact.value}</Text>
                    </View>
                  ))}
                </View>
              )}

              <View style={styles.section}>
                <Text style={styles.sectionTitle}>YOUR 30-DAY PATTERN</Text>
                <View style={styles.factRow}>
                  <Text style={styles.factLabel}>Most recent 15 days</Text>
                  <Text style={styles.factValue}>{formatUSD(pattern.recentSpentMinor)}</Text>
                </View>
                <View style={styles.factRow}>
                  <Text style={styles.factLabel}>Previous 15 days</Text>
                  <Text style={styles.factValue}>{formatUSD(pattern.previousSpentMinor)}</Text>
                </View>
                <View style={styles.factRow}>
                  <Text style={styles.factLabel}>Trend</Text>
                  <Text style={styles.factValue}>{changeLabels[pattern.change]}</Text>
                </View>
                {(pattern.peakDay || pattern.peakTime) && (
                  <View style={styles.factRow}>
                    <Text style={styles.factLabel}>Most active period</Text>
                    <Text style={styles.factValue}>{[pattern.peakDay, pattern.peakTime].filter(Boolean).join(' · ')}</Text>
                  </View>
                )}
              </View>

              {!!pattern.frequentReasons?.length && (
                <View style={styles.section}>
                  <Text style={styles.sectionTitle}>COMMON REASONS</Text>
                  {pattern.frequentReasons.map((item) => (
                    <View key={item.reason} style={styles.factRow}>
                      <Text style={styles.factLabel}>{item.reason} · {item.count}×</Text>
                      <Text style={styles.factValue}>{formatUSD(item.amountMinor)}</Text>
                    </View>
                  ))}
                </View>
              )}
            </ScrollView>

            <View style={styles.actionWrap}>
              <ActionButton label="Close and continue" onPress={close} />
            </View>
          </View>
        </View>
      </Modal>
    </>
  );
}

const styles = StyleSheet.create({
  actionWrap: { paddingHorizontal: 20, paddingTop: 12 },
  close: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 30, lineHeight: 30 },
  content: { paddingBottom: 4 },
  eyebrow: { color: colors.blue, fontFamily: fonts.bold, fontSize: 8, letterSpacing: 0.7 },
  factLabel: { color: colors.textMuted, flex: 1, fontFamily: fonts.medium, fontSize: 12, lineHeight: 17, paddingRight: 14 },
  factRow: { alignItems: 'flex-start', borderBottomColor: colors.border, borderBottomWidth: StyleSheet.hairlineWidth, flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 9 },
  factValue: { color: colors.navy, flex: 1, fontFamily: fonts.bold, fontSize: 12, lineHeight: 17, textAlign: 'right' },
  handle: { alignSelf: 'center', backgroundColor: colors.border, borderRadius: 2, height: 4, marginBottom: 13, width: 38 },
  headingRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' },
  message: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, lineHeight: 21, marginTop: 8 },
  modalEyebrow: { color: colors.blue, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 0.9 },
  modalRoot: { flex: 1, justifyContent: 'flex-end' },
  pressed: { opacity: 0.76 },
  relationshipBadge: { alignSelf: 'flex-start', backgroundColor: colors.blueTint, borderRadius: radii.sm, marginTop: 14, paddingHorizontal: 10, paddingVertical: 7 },
  relationshipText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 11 },
  scrim: { backgroundColor: 'rgba(15, 27, 50, 0.46)', bottom: 0, left: 0, position: 'absolute', right: 0, top: 0 },
  section: { marginTop: 20 },
  sectionTitle: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 9, letterSpacing: 0.9 },
  sheet: { backgroundColor: colors.white, borderTopLeftRadius: 24, borderTopRightRadius: 24, maxHeight: '84%', paddingBottom: 20, paddingHorizontal: 20, paddingTop: 10 },
  status: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginHorizontal: 40, marginTop: 7, textAlign: 'center' },
  title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 21, lineHeight: 27, marginTop: 5 },
  trigger: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: radii.sm, flexDirection: 'row', justifyContent: 'space-between', marginHorizontal: 36, marginTop: 8, paddingHorizontal: 11, paddingVertical: 9 },
  triggerCopy: { flex: 1 },
  triggerTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12, marginTop: 3 },
  viewLabel: { color: colors.blue, fontFamily: fonts.bold, fontSize: 9, letterSpacing: 0.7, marginLeft: 12 },
  warning: { backgroundColor: colors.orangeTint },
});
