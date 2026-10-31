import { useEffect } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSequence, withSpring, withTiming } from 'react-native-reanimated';
import { PressableScale, springs } from '@/motion';
import { colors, fonts } from '@/theme';
type Props = { calculation?: string | null; caption?: string; onCalculationPress?: () => void; tone?: 'default' | 'danger'; value: string };
export function MoneyAmount({ calculation, caption, onCalculationPress, tone = 'default', value }: Props) {
  const bump = useSharedValue(1);
  useEffect(() => { bump.set(withSequence(withTiming(1.05, { duration: 70 }), withSpring(1, springs.press))); }, [value, bump]);
  const bumpStyle = useAnimatedStyle(() => ({ transform: [{ scale: bump.value }] }));
  return <View style={styles.wrap}><Animated.View style={[styles.valueBox, bumpStyle]}><Text adjustsFontSizeToFit numberOfLines={1} style={[styles.value, tone === 'danger' && styles.danger]}>${value || '0'}</Text></Animated.View>{calculation ? <PressableScale accessibilityLabel={`Use calculated amount ${calculation}`} accessibilityRole="button" onPress={onCalculationPress} scaleTo={0.94} style={styles.calculationButton}><Text style={styles.calculation}>= ${calculation}</Text></PressableScale> : caption ? <Text style={styles.caption}>{caption}</Text> : null}</View>;
}
const styles = StyleSheet.create({ wrap: { alignItems: 'center', minHeight: 120, justifyContent: 'center' }, value: { color: colors.navy, fontFamily: fonts.bold, fontSize: 60, letterSpacing: -2, maxWidth: '100%' }, danger: { color: colors.redText }, caption: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 4 }, calculationButton: { backgroundColor: colors.softButton, borderRadius: 10, marginTop: 4, paddingHorizontal: 14, paddingVertical: 6 }, calculation: { color: colors.keyText, fontFamily: fonts.bold, fontSize: 14 }, valueBox: { maxWidth: '100%' } });
