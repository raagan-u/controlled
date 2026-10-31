import type { ReactNode } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { colors, fonts, radii } from '@/theme';

export function InfoBox({ children, tone = 'blue' }: { children: ReactNode; tone?: 'blue' | 'orange' | 'red' }) {
  const palette = tone === 'red' ? { bg: colors.redTint, text: colors.redText } : tone === 'orange' ? { bg: colors.orangeTint, text: colors.orangeText } : { bg: colors.surface, text: colors.textMuted };
  return <View style={[styles.box, { backgroundColor: palette.bg }]}><Text style={[styles.icon, { color: palette.text }]}>ⓘ</Text><Text style={[styles.text, { color: palette.text }]}>{children}</Text></View>;
}

const styles = StyleSheet.create({ box: { alignItems: 'flex-start', borderRadius: radii.md, flexDirection: 'row', gap: 9, marginVertical: 10, padding: 12 }, icon: { fontFamily: fonts.bold, fontSize: 13 }, text: { flex: 1, fontFamily: fonts.medium, fontSize: 12, lineHeight: 17 } });
