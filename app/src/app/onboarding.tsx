import { useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';

import { colors, fonts } from '@/theme';

export default function OnboardingScreen() {
  const [isFocused, setIsFocused] = useState(false);
  const [name, setName] = useState('');

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
          returnKeyType="done"
          selectionColor={colors.primary}
          style={[styles.nameInput, isFocused && styles.nameInputFocused]}
          value={name}
        />
        <Text style={styles.greetingText}>!</Text>
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
