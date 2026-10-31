import { router } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Platform, StyleSheet, Text, TextInput, useWindowDimensions, View } from 'react-native';
import Animated, { FadeInDown, LinearTransition } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import { apiErrorMessage, updateDisplayName } from '@/api/wallet';
import { ErrorBanner } from '@/components/ErrorBanner';
import { PrimaryButton } from '@/components/PrimaryButton';
import { rise } from '@/motion';
import { colors, fonts } from '@/theme';

const baseSize = 44;
const minSize = 22;
const gap = 12;
const move = LinearTransition.springify().damping(20).stiffness(180);

// First launch after sign-up: the user picks the name others will see.
export default function HelloScreen() {
  const { width } = useWindowDimensions();
  const [name, setName] = useState('');
  const [helloWidth, setHelloWidth] = useState(130);
  const [textWidth, setTextWidth] = useState(0);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const trimmed = name.replace(/\s+/g, ' ').trim();

  // The name sits beside "Hello," while it fits; once it would not, the input drops below it and
  // takes the full width, shrinking a little for very long names so nothing is ever cut off.
  const blockWidth = Math.min(width - 48, 360);
  const inlineWidth = blockWidth - helloWidth - gap;
  const typedWidth = name ? textWidth : 0;
  const below = typedWidth + 6 > inlineWidth;
  const fontSize = below ? Math.max(minSize, Math.min(baseSize, (baseSize * (blockWidth - 6)) / Math.max(typedWidth, 1))) : baseSize;

  const save = async () => {
    if (!trimmed || saving) return;
    setError(undefined);
    setSaving(true);
    try {
      await updateDisplayName(trimmed);
      router.replace('/home');
    } catch (caught) {
      setError(apiErrorMessage(caught));
      setSaving(false);
    }
  };

  return (
    <SafeAreaView edges={['top', 'bottom']} style={styles.safeArea}>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={styles.flex}>
        <View style={styles.center}>
          <Animated.View entering={rise(0, 120)} layout={move} style={[styles.row, { width: blockWidth }]}>
            <Text onLayout={(event) => setHelloWidth(event.nativeEvent.layout.width)} style={styles.text}>Hello,</Text>
            <Animated.View layout={move} style={{ width: below ? blockWidth : inlineWidth }}>
              <TextInput
                autoCapitalize="words"
                autoComplete="name"
                autoCorrect={false}
                autoFocus
                maxLength={60}
                onChangeText={setName}
                onSubmitEditing={() => void save()}
                placeholder="name"
                placeholderTextColor={colors.grabber}
                returnKeyType="done"
                selectionColor={colors.primary}
                style={[styles.text, styles.input, { fontSize, lineHeight: Math.round(fontSize * 1.27) }]}
                value={name}
              />
            </Animated.View>
          </Animated.View>
          {/* Hidden single-line mirror: reports how wide the typed name really is at full size. */}
          <Text onTextLayout={(event) => setTextWidth(event.nativeEvent.lines[0]?.width ?? 0)} pointerEvents="none" style={[styles.text, styles.mirror]}>{name}</Text>
        </View>
        <View style={styles.footer}>
          {error ? <ErrorBanner message={error} /> : null}
          {trimmed ? <Animated.View entering={FadeInDown.duration(260)} style={styles.button}><PrimaryButton disabled={saving} label="Continue" onPress={() => void save()} /></Animated.View> : <View style={styles.buttonSlot} />}
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  safeArea: { backgroundColor: colors.background, flex: 1 },
  center: { alignItems: 'center', flex: 1, justifyContent: 'center', paddingHorizontal: 24 },
  row: { alignItems: 'center', columnGap: gap, flexDirection: 'row', flexWrap: 'wrap' },
  text: { color: colors.ink, fontFamily: fonts.black, fontSize: baseSize, letterSpacing: -2, lineHeight: 56 },
  input: { color: colors.primary, padding: 0 },
  mirror: { left: 0, opacity: 0, position: 'absolute', top: 0, width: 4000 },
  footer: { paddingBottom: 12, paddingHorizontal: 18 },
  button: { marginTop: 14 },
  buttonSlot: { height: 66, marginTop: 14 },
});
