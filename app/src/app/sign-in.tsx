import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AuthFormScreen } from '@/components/AuthFormScreen';
import { useGoogleAuthentication } from '@/hooks/useGoogleAuthentication';
import { usePasswordAuthentication } from '@/hooks/usePasswordAuthentication';
import { colors, fonts } from '@/theme';

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function validateSignIn(values: Record<string, string>) {
  const errors: Record<string, string> = {};

  if (!emailPattern.test(values.email?.trim() ?? '')) {
    errors.email = 'Enter a valid email address';
  }
  if (!(values.password?.trim() ?? '')) {
    errors.password = 'Enter your password';
  }

  return errors;
}

export default function SignInScreen() {
  const google = useGoogleAuthentication();
  const password = usePasswordAuthentication('login');

  return (
    <AuthFormScreen
      fields={[
        { autoComplete: 'email', keyboardType: 'email-address', label: 'email', name: 'email' },
        {
          autoComplete: 'password',
          label: 'password',
          name: 'password',
          secureTextEntry: true,
        },
      ]}
      footer={
        <View style={styles.row}>
          <Text style={styles.prompt}>Don’t have an account? </Text>
          <Pressable accessibilityRole="link" onPress={() => router.replace('/sign-up')}>
            <Text style={styles.link}>Sign up</Text>
          </Pressable>
        </View>
      }
      formError={password.error}
      googleError={google.error}
      googleLoading={google.loading}
      onGooglePress={google.authenticate}
      onSubmit={password.authenticate}
      submitting={password.loading}
      submitLabel="sign in"
      title="sign in"
      validate={validateSignIn}
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
