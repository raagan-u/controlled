import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AuthFormScreen } from '@/components/AuthFormScreen';
import { colors, fonts } from '@/theme';

export default function SignInScreen() {
  return (
    <AuthFormScreen
      fields={[
        { autoComplete: 'email', keyboardType: 'email-address', label: 'email' },
        { autoComplete: 'password', label: 'password', secureTextEntry: true },
      ]}
      footer={
        <View style={styles.row}>
          <Text style={styles.prompt}>Don’t have an account? </Text>
          <Pressable accessibilityRole="link" onPress={() => router.replace('/sign-up')}>
            <Text style={styles.link}>Sign in</Text>
          </Pressable>
        </View>
      }
      onGooglePress={() => {}}
      onSubmit={() => router.push('/onboarding')}
      submitLabel="sign in"
      title="sign in"
    />
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
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
