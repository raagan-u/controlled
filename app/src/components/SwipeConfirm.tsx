import { useState } from 'react';
import { Animated, PanResponder, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { colors, fonts, radii } from '@/theme';
type Props = { disabled?: boolean; label?: string; onConfirm: () => void };
export function SwipeConfirm({ disabled, label = 'Swipe right to confirm', onConfirm }: Props) {
  const { width } = useWindowDimensions(); const [x] = useState(() => new Animated.Value(0)); const [done, setDone] = useState(false); const limit = Math.max(160, width - 92);
  const pan = PanResponder.create({ onMoveShouldSetPanResponder: (_, g) => !disabled && Math.abs(g.dx) > 6, onPanResponderMove: (_, g) => x.setValue(Math.max(0, Math.min(g.dx, limit))), onPanResponderRelease: (_, g) => { if (g.dx > limit * 0.6) { setDone(true); Animated.timing(x, { duration: 160, toValue: limit, useNativeDriver: true }).start(onConfirm); } else Animated.spring(x, { toValue: 0, useNativeDriver: true }).start(); } });
  return <View accessibilityLabel={label} style={[styles.track, disabled && styles.disabled]}><Text style={styles.hint}>{done ? 'Confirmed' : label}</Text><Animated.View {...pan.panHandlers} style={[styles.thumb, { transform: [{ translateX: x }] }]}><Text style={styles.arrow}>›</Text></Animated.View></View>;
}
const styles = StyleSheet.create({ track: { alignItems: 'center', backgroundColor: colors.softButton, borderRadius: radii.md, height: 60, justifyContent: 'center', overflow: 'hidden' }, thumb: { alignItems: 'center', backgroundColor: colors.navy, borderRadius: 10, height: 52, justifyContent: 'center', left: 4, position: 'absolute', width: 52 }, arrow: { color: colors.white, fontFamily: fonts.medium, fontSize: 34, lineHeight: 36 }, hint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15 }, disabled: { opacity: 0.45 } });
