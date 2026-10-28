import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { colors, fonts } from '@/theme';

export default function OnboardingScreen() {
  const [isFocused, setIsFocused] = useState(false);
  const [name, setName] = useState('');
  const normalizedName = name.trim();

  const continueToHome = () => {
    if (normalizedName) {
      router.replace({ pathname: '/home', params: { name: normalizedName } });
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.greeting}>
        <Text style={styles.greetingText}>Hello,</Text>
        <TextInput
          accessibilityLabel="Your name"
          autoCapitalize="words"
          autoComplete="name"
          maxLength={24}
          onBlur={() => setIsFocused(false)}
          onChangeText={setName}
          onFocus={() => setIsFocused(true)}
          onSubmitEditing={continueToHome}
          returnKeyType="done"
          selectionColor={colors.primary}
          style={[styles.nameInput, isFocused && styles.nameInputFocused]}
          value={name}
        />
        <Text style={styles.greetingText}>!</Text>
      </View>
      <View style={styles.actionSlot}>
        {normalizedName ? (
          <Pressable
            accessibilityLabel="Continue to home"
            accessibilityRole="button"
            onPress={continueToHome}
            style={({ pressed }) => [styles.continueButton, pressed && styles.continuePressed]}
          >
            <Text style={styles.continueIcon}>&gt;</Text>
          </Pressable>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    backgroundColor: colors.background,
    flex: 1,
    justifyContent: 'center',
    paddingHorizontal: 20,
  },
  greeting: {
    alignItems: 'baseline',
    flexDirection: 'row',
    justifyContent: 'center',
  },
  actionSlot: {
    alignItems: 'center',
    height: 58,
    marginTop: 28,
  },
  continueButton: {
    alignItems: 'center',
    backgroundColor: colors.primary,
    borderRadius: 29,
    height: 58,
    justifyContent: 'center',
    width: 58,
  },
  continueIcon: {
    color: colors.background,
    fontFamily: fonts.bold,
    fontSize: 29,
    lineHeight: 34,
    marginLeft: 2,
  },
  continuePressed: {
    opacity: 0.84,
    transform: [{ scale: 0.96 }],
  },
  greetingText: {
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 54,
    letterSpacing: -3.2,
    lineHeight: 66,
    textAlign: 'center',
  },
  nameInput: {
    borderBottomColor: colors.muted,
    borderBottomWidth: 1,
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 48,
    height: 62,
    letterSpacing: -2.6,
    marginHorizontal: 8,
    paddingBottom: 2,
    paddingHorizontal: 4,
    textAlign: 'center',
    width: 132,
  },
  nameInputFocused: {
    borderBottomColor: colors.primary,
  },
});
