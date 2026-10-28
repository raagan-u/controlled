import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AuthFormScreen } from '@/components/AuthFormScreen';
import { useGoogleAuthentication } from '@/hooks/useGoogleAuthentication';
import { colors, fonts } from '@/theme';

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function validateSignUp(values: Record<string, string>) {
  const errors: Record<string, string> = {};

  if (!emailPattern.test(values.email?.trim() ?? '')) {
    errors.email = 'Enter a valid email address';
  }
  if (!(values.password?.trim() ?? '')) {
    errors.password = 'Enter a password';
  }
  if (!(values.confirmPassword?.trim() ?? '')) {
    errors.confirmPassword = 'Confirm your password';
  } else if (values.confirmPassword !== values.password) {
    errors.confirmPassword = 'Passwords do not match';
  }

  return errors;
}

export default function SignUpScreen() {
  const google = useGoogleAuthentication();

  return (
    <AuthFormScreen
      fields={[
        { autoComplete: 'email', keyboardType: 'email-address', label: 'email', name: 'email' },
        {
          autoComplete: 'new-password',
          label: 'password',
          name: 'password',
          secureTextEntry: true,
        },
        {
          autoComplete: 'new-password',
          label: 'confirm password',
          name: 'confirmPassword',
          secureTextEntry: true,
        },
      ]}
      footer={
        <View style={styles.row}>
          <Text style={styles.prompt}>Already have an account? </Text>
          <Pressable accessibilityRole="link" onPress={() => router.replace('/sign-in')}>
            <Text style={styles.link}>Sign in</Text>
          </Pressable>
        </View>
      }
      googleError={google.error}
      googleLoading={google.loading}
      onGooglePress={google.authenticate}
      onSubmit={() => router.push('/verify-email')}
      submitLabel="Create account"
      title="sign up"
      validate={validateSignUp}
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
