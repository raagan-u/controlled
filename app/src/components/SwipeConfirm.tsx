import { useState } from 'react';
import { Animated, PanResponder, StyleSheet, Text, useWindowDimensions } from 'react-native';
import { colors, fonts } from '@/theme';
type Props = { disabled?: boolean; label?: string; onConfirm: () => void };
export function SwipeConfirm({ disabled, label = 'Swipe right to confirm', onConfirm }: Props) {
  const { width } = useWindowDimensions(); const [x] = useState(() => new Animated.Value(0)); const [done, setDone] = useState(false); const limit = Math.max(160, width - 92);
  const pan = PanResponder.create({ onMoveShouldSetPanResponder: (_, g) => !disabled && Math.abs(g.dx) > 6, onPanResponderMove: (_, g) => x.setValue(Math.max(0, Math.min(g.dx, limit))), onPanResponderRelease: (_, g) => { if (g.dx > limit * 0.6) { setDone(true); Animated.timing(x, { duration: 160, toValue: limit, useNativeDriver: true }).start(onConfirm); } else Animated.spring(x, { toValue: 0, useNativeDriver: true }).start(); } });
  return <Animated.View {...pan.panHandlers} accessibilityLabel={label} style={[styles.track, disabled && styles.disabled, { transform: [{ translateX: x.interpolate({ inputRange: [0, limit], outputRange: [0, 18], extrapolate: 'clamp' }) }] }]}><Text style={[styles.hand, done && styles.active]}>☝︎</Text><Text style={[styles.hint, done && styles.active]}>{done ? 'Keep going' : label}</Text></Animated.View>;
}
const styles = StyleSheet.create({ track: { alignItems: 'center', alignSelf: 'center', height: 88, justifyContent: 'center', width: 240 }, hand: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 31 }, hint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15, marginTop: 5 }, active: { color: colors.blue }, disabled: { opacity: 0.35 } });
