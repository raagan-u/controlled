import { router } from 'expo-router';
import { Pressable, StyleSheet, Text } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { colors, fonts } from '@/theme';

export default function VerifyEmailScreen() {
  return (
    <SafeAreaView edges={['top', 'bottom']} style={styles.safeArea}>
      <Pressable
        accessibilityHint="Continue to onboarding"
        accessibilityRole="button"
        onPress={() => router.push('/onboarding')}
        style={styles.fill}
      >
        <Text style={styles.title}>Let’s verify ur email!</Text>
      </Pressable>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    backgroundColor: colors.background,
    flex: 1,
  },
  fill: {
    flex: 1,
    paddingHorizontal: 18,
    paddingTop: 44,
  },
  title: {
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 42,
    letterSpacing: -2.4,
    lineHeight: 50,
  },
});
