import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, type PressableProps, type StyleProp, type ViewStyle } from 'react-native';
import Animated, { FadeInDown, FadeInUp, ZoomIn, useAnimatedStyle, useSharedValue, withDelay, withRepeat, withSequence, withSpring, withTiming, type AnimatedProps } from 'react-native-reanimated';

export const springs = {
  press: { damping: 18, mass: 0.6, stiffness: 420 },
  soft: { damping: 20, mass: 0.9, stiffness: 180 },
} as const;

// Staggered rise-in used for lists and stacked sections.
export const rise = (index = 0, base = 0) => FadeInDown.delay(base + Math.min(index, 8) * 55).duration(420).springify().damping(18).stiffness(160);
export const drop = (index = 0, base = 0) => FadeInUp.delay(base + index * 55).duration(420).springify().damping(18);
export const pop = (delay = 0) => ZoomIn.delay(delay).springify().damping(11).stiffness(170);

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

type Props = Omit<PressableProps, 'style'> & Pick<AnimatedProps<PressableProps>, 'entering' | 'layout'> & { scaleTo?: number; style?: StyleProp<ViewStyle> };

const outerKeys = new Set(['margin', 'marginTop', 'marginBottom', 'marginLeft', 'marginRight', 'marginHorizontal', 'marginVertical', 'marginStart', 'marginEnd', 'flex', 'flexGrow', 'flexShrink', 'flexBasis', 'width', 'height', 'minWidth', 'minHeight', 'maxWidth', 'maxHeight', 'alignSelf', 'position', 'top', 'left', 'right', 'bottom', 'zIndex']);

// Reanimated can't run an entrance animation and a press transform on the same view, so with
// `entering`/`layout` an outer view takes the sizing/margins and plays the entrance.
function splitStyle(style: StyleProp<ViewStyle>) {
  const flat = (StyleSheet.flatten(style) ?? {}) as Record<string, unknown>;
  const outer: Record<string, unknown> = {};
  const inner: Record<string, unknown> = {};
  for (const key of Object.keys(flat)) (outerKeys.has(key) ? outer : inner)[key] = flat[key];
  return { inner: { ...inner, flexGrow: 1 } as ViewStyle, outer: outer as ViewStyle };
}

// Pressable that springs down on touch and back on release.
export function PressableScale({ disabled, entering, layout, onPressIn, onPressOut, scaleTo = 0.96, style, ...rest }: Props) {
  const scale = useSharedValue(1);
  const animated = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  const wrapped = Boolean(entering || layout);
  const { inner, outer } = wrapped ? splitStyle(style) : { inner: style, outer: undefined };
  const pressable = <AnimatedPressable
    {...rest}
    disabled={disabled}
    onPressIn={(event) => { scale.set(withSpring(scaleTo, springs.press)); onPressIn?.(event); }}
    onPressOut={(event) => { scale.set(withSpring(1, springs.press)); onPressOut?.(event); }}
    style={[inner, animated]}
  />;
  return wrapped ? <Animated.View entering={entering} layout={layout} style={outer}>{pressable}</Animated.View> : pressable;
}

// Eases a number from its previous value to the target (JS-driven; for text that can't run on the UI thread).
export function useCountUp(target: number | undefined, duration = 700) {
  const [value, setValue] = useState(target ?? 0);
  const from = useRef(target ?? 0);
  useEffect(() => {
    if (target === undefined) return;
    const start = from.current;
    if (start === target) { setValue(target); return; }
    const begin = Date.now();
    let frame = requestAnimationFrame(function tick() {
      const t = Math.min(1, (Date.now() - begin) / duration);
      const next = start + (target - start) * (1 - Math.pow(1 - t, 3));
      from.current = next;
      setValue(next);
      if (t < 1) frame = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(frame);
  }, [target, duration]);
  return value;
}

// Wraps a field so it shakes whenever `error` changes to a new message.
export function ShakeOnError({ amplitude = 8, children, delay = 0, error, style }: { amplitude?: number; children: ReactNode; delay?: number; error?: string; style?: StyleProp<ViewStyle> }) {
  const x = useSharedValue(0);
  useEffect(() => {
    if (!error) return;
    x.set(withDelay(delay, withSequence(withTiming(-amplitude, { duration: 50 }), withRepeat(withTiming(amplitude, { duration: 80 }), 4, true), withTiming(0, { duration: 50 }))));
  }, [amplitude, delay, error, x]);
  const animated = useAnimatedStyle(() => ({ transform: [{ translateX: x.value }] }));
  return <Animated.View style={[style, animated]}>{children}</Animated.View>;
}
