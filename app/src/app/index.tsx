import { useCallback, useEffect, useRef, useState } from 'react';
import { router, type Href } from 'expo-router';
import * as SecureStore from 'expo-secure-store';
import { AccessibilityInfo, Pressable, StyleSheet, Text, View } from 'react-native';
import Animated, { Easing, FadeInDown, LinearTransition, useAnimatedStyle, useSharedValue, withRepeat, withTiming } from 'react-native-reanimated';

import { signedInRoute } from '@/auth/routing';
import { getSessionToken } from '@/auth/session';
import { colors, fonts } from '@/theme';

const word = 'ctrld';
// The stressed word is highlighted; swap the three parts to change the line.
const tagline = { before: 'a wallet that ', stress: 'actually', after: ' helps you stick to your budget.' };
const splashSeenKey = 'controlled_splash_seen';
// From the slide starting: the entry is fully in by ~0.9s, then stays up long enough to read comfortably.
const entryHold = 4800;

type Plan = { full: boolean; instant: boolean; route: Promise<Href> };

// Resolves where to go, never waiting on the network for more than a few seconds.
function resolveRoute(token: string | null): Promise<Href> {
  if (!token) return Promise.resolve('/welcome');
  return Promise.race([signedInRoute(), new Promise<Href>((resolve) => setTimeout(() => resolve('/home'), 3000))]);
}

// Full sequence (type, slide left, dictionary entry) for first launch and signed-out users; a short
// type-and-go for returning signed-in users. Tapping anywhere skips.
export default function SplashScreen() {
  const [plan, setPlan] = useState<Plan | null>(null);
  const [typed, setTyped] = useState('');
  const [leftAligned, setLeftAligned] = useState(false);
  const [entry, setEntry] = useState(false);
  const left = useRef(false);
  const caret = useSharedValue(1);
  const caretStyle = useAnimatedStyle(() => ({ opacity: caret.value }));

  useEffect(() => {
    let active = true;
    void Promise.all([getSessionToken(), SecureStore.getItemAsync(splashSeenKey).catch(() => null), AccessibilityInfo.isReduceMotionEnabled()]).then(([token, seen, reduce]) => {
      if (active) setPlan({ full: !token || !seen, instant: reduce, route: resolveRoute(token) });
    });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    caret.set(withRepeat(withTiming(0, { duration: 480, easing: Easing.inOut(Easing.quad) }), -1, true));
  }, [caret]);

  const go = useCallback((target: Plan) => {
    if (left.current) return;
    left.current = true;
    if (target.full) void SecureStore.setItemAsync(splashSeenKey, '1').catch(() => undefined);
    void target.route.then((route) => router.replace(route));
  }, []);

  useEffect(() => {
    if (!plan) return;
    const timers: ReturnType<typeof setTimeout>[] = [];
    const at = (ms: number, run: () => void) => { timers.push(setTimeout(run, ms)); };
    const typeDelay = plan.instant ? 0 : 300;
    const letterGap = plan.instant ? 0 : 120;
    for (let index = 1; index <= word.length; index += 1) at(typeDelay + index * letterGap, () => setTyped(word.slice(0, index)));
    const typedAt = typeDelay + word.length * letterGap;
    if (plan.full && !plan.instant) {
      at(typedAt + 700, () => { setLeftAligned(true); setEntry(true); });
      at(typedAt + 700 + entryHold, () => go(plan));
    } else {
      at(typedAt + 500, () => go(plan));
    }
    return () => timers.forEach(clearTimeout);
  }, [go, plan]);

  return (
    <Pressable accessibilityLabel="Controlled" accessibilityRole="button" accessibilityHint="Skips the intro" disabled={!plan} onPress={() => { if (plan) go(plan); }} style={styles.container}>
      <Animated.View layout={LinearTransition.duration(600).easing(Easing.out(Easing.cubic))} style={[styles.mark, leftAligned ? styles.markLeft : styles.markCenter]}>
        {/* The full word is laid out invisibly so the line doesn't re-center as letters appear. */}
        <Text style={[styles.word, styles.ghost]}>{word}</Text>
        <View style={styles.typedRow}>
          <Text style={styles.word}>{typed}</Text>
          <Animated.Text style={[styles.word, styles.caret, caretStyle]}>_</Animated.Text>
        </View>
      </Animated.View>
      {entry ? (
        <View style={styles.entry}>
          <Animated.Text entering={FadeInDown.delay(150).duration(400)} style={styles.pronunciation}>/kənˈtroʊld/  ·  kuhn-TROHLD</Animated.Text>
          <Animated.Text entering={FadeInDown.delay(300).duration(400)} style={styles.partOfSpeech}>adj.</Animated.Text>
          <Animated.Text entering={FadeInDown.delay(450).duration(400)} style={styles.tagline}>{tagline.before}<Text style={styles.stress}>{tagline.stress}</Text>{tagline.after}</Animated.Text>
        </View>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: { backgroundColor: colors.background, flex: 1, justifyContent: 'center', paddingHorizontal: 28 },
  mark: { flexDirection: 'row' },
  markCenter: { alignSelf: 'center' },
  markLeft: { alignSelf: 'flex-start' },
  word: { color: colors.ink, fontFamily: fonts.black, fontSize: 64, letterSpacing: -5, lineHeight: 75 },
  ghost: { opacity: 0 },
  typedRow: { flexDirection: 'row', left: 0, position: 'absolute', top: 0 },
  caret: { marginLeft: 2 },
  entry: { alignSelf: 'flex-start', marginTop: 10 },
  pronunciation: { color: colors.muted, fontFamily: fonts.medium, fontSize: 17, letterSpacing: 0.2 },
  partOfSpeech: { color: colors.primary, fontFamily: fonts.bold, fontSize: 15, marginTop: 22 },
  stress: { color: colors.primary },
  tagline: { color: colors.ink, fontFamily: fonts.bold, fontSize: 24, letterSpacing: -0.5, lineHeight: 31, marginTop: 4, maxWidth: 300 },
});
