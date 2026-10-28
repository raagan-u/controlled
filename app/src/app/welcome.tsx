import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Brand } from '@/components/Brand';
import { PrimaryButton } from '@/components/PrimaryButton';
import { colors, fonts } from '@/theme';

export default function WelcomeScreen() {
  return (
    <SafeAreaView edges={['top', 'bottom']} style={styles.safeArea}>
      <View style={styles.brandBlock}>
        <Brand />
        <Text style={styles.tagline}>Your money, moving with purpose.</Text>
      </View>

      <View style={styles.actions}>
        <PrimaryButton label="Create account" onPress={() => router.push('/sign-up')} />
        <View style={styles.signInRow}>
          <Text style={styles.prompt}>Already have an account? </Text>
          <Pressable accessibilityRole="link" onPress={() => router.push('/sign-in')}>
            <Text style={styles.link}>Sign in</Text>
          </Pressable>
        </View>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    backgroundColor: colors.background,
    flex: 1,
    justifyContent: 'space-between',
    paddingHorizontal: 18,
  },
  brandBlock: {
    alignItems: 'center',
    marginTop: '5%',
  },
  tagline: {
    color: colors.muted,
    fontFamily: fonts.regular,
    fontSize: 17,
    letterSpacing: 0.1,
    lineHeight: 24,
    marginTop: -5,
  },
  actions: {
    alignItems: 'center',
    marginBottom: 7,
    width: '100%',
  },
  signInRow: {
    flexDirection: 'row',
    marginTop: 15,
  },
  prompt: {
    color: colors.muted,
    fontFamily: fonts.regular,
    fontSize: 16,
    lineHeight: 22,
  },
  link: {
    color: colors.primary,
    fontFamily: fonts.bold,
    fontSize: 16,
    lineHeight: 22,
  },
});
