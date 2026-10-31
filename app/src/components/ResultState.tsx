import type { ReactNode } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import Animated from 'react-native-reanimated';
import { pop, rise } from '@/motion';
import { colors, fonts } from '@/theme';
type Tone = 'success' | 'pending' | 'failed';
type Props = { amount?: string; children?: ReactNode; detail: string; title: string; tone?: Tone };
export function ResultState({ amount, children, detail, title, tone = 'success' }: Props) {
  const symbol = tone === 'success' ? '✓' : tone === 'failed' ? '!' : '…';
  return <View style={styles.wrap}><Animated.View entering={pop(80)} style={[styles.icon, styles[`${tone}Icon`]]}><Text style={[styles.symbol, styles[`${tone}Text`]]}>{symbol}</Text></Animated.View><Animated.Text entering={rise(0, 220)} style={styles.title}>{title}</Animated.Text>{amount ? <Animated.Text entering={rise(1, 220)} style={styles.amount}>${amount}</Animated.Text> : null}<Animated.Text entering={rise(2, 220)} style={styles.detail}>{detail}</Animated.Text>{children ? <Animated.View entering={rise(3, 220)}>{children}</Animated.View> : null}</View>;
}
const styles = StyleSheet.create({ wrap: { alignItems: 'center', flex: 1, paddingHorizontal: 10, paddingTop: 48 }, icon: { alignItems: 'center', borderRadius: 36, height: 72, justifyContent: 'center', width: 72 }, successIcon: { backgroundColor: colors.greenTint }, pendingIcon: { backgroundColor: colors.orangeTint }, failedIcon: { backgroundColor: colors.redTint }, symbol: { fontFamily: fonts.bold, fontSize: 32 }, successText: { color: colors.greenText }, pendingText: { color: colors.orangeText }, failedText: { color: colors.redText }, title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6, marginTop: 20, textAlign: 'center' }, amount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 48, letterSpacing: -1.5, marginTop: 12 }, detail: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15, lineHeight: 22, marginTop: 10, maxWidth: 290, textAlign: 'center' } });
