import { useEffect, useState } from 'react';
import { PanResponder, StyleSheet, Text, useWindowDimensions } from 'react-native';
import Animated, { Easing, interpolate, runOnJS, useAnimatedStyle, useSharedValue, withDelay, withRepeat, withSequence, withSpring, withTiming } from 'react-native-reanimated';
import { springs } from '@/motion';
import { colors, fonts } from '@/theme';
type Props = { disabled?: boolean; label?: string; onConfirm: () => void };
export function SwipeConfirm({ disabled, label = 'Swipe right to confirm', onConfirm }: Props) {
  const { width } = useWindowDimensions(); const x = useSharedValue(0); const nudge = useSharedValue(0); const [done, setDone] = useState(false); const limit = Math.max(160, width - 92);
  // Idle hint: a gentle repeated nudge to the right until the user touches it.
  useEffect(() => {
    if (disabled) return;
    nudge.set(withRepeat(withDelay(1400, withSequence(withTiming(10, { duration: 420, easing: Easing.out(Easing.cubic) }), withSpring(0, springs.soft))), -1));
    return () => nudge.set(0);
  }, [disabled, nudge]);
  const pan = PanResponder.create({
    onMoveShouldSetPanResponder: (_, g) => !disabled && Math.abs(g.dx) > 6,
    onPanResponderGrant: () => nudge.set(0),
    onPanResponderMove: (_, g) => x.set(Math.max(0, Math.min(g.dx, limit))),
    onPanResponderRelease: (_, g) => {
      if (g.dx > limit * 0.6) { setDone(true); x.set(withTiming(limit, { duration: 160 }, (finished) => { if (finished) runOnJS(onConfirm)(); })); }
      else x.set(withSpring(0, springs.soft));
    },
  });
  const trackStyle = useAnimatedStyle(() => ({ opacity: interpolate(x.value, [0, limit], [1, 0.55], 'clamp'), transform: [{ translateX: interpolate(x.value, [0, limit], [0, 18], 'clamp') + nudge.value }] }));
  return <Animated.View {...pan.panHandlers} accessibilityLabel={label} style={[styles.track, disabled ? styles.disabled : trackStyle]}><Text style={[styles.hint, done && styles.active]}>{done ? 'Keep going' : label}</Text></Animated.View>;
}
const styles = StyleSheet.create({ track: { alignItems: 'center', alignSelf: 'center', height: 62, justifyContent: 'center', width: 240 }, hint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 15 }, active: { color: colors.blue }, disabled: { opacity: 0.35 } });
