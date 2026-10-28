import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AuthFormScreen } from '@/components/AuthFormScreen';
import { colors, fonts } from '@/theme';

export default function SignUpScreen() {
  return (
    <AuthFormScreen
      fields={[
        { autoComplete: 'email', keyboardType: 'email-address', label: 'email' },
        { autoComplete: 'new-password', label: 'password', secureTextEntry: true },
        { autoComplete: 'new-password', label: 'confirm password', secureTextEntry: true },
      ]}
      footer={
        <View style={styles.row}>
          <Text style={styles.prompt}>Already have an account? </Text>
          <Pressable accessibilityRole="link" onPress={() => router.replace('/sign-in')}>
            <Text style={styles.link}>Sign in</Text>
          </Pressable>
        </View>
      }
      onGooglePress={() => {}}
      onSubmit={() => router.push('/verify-email')}
      submitLabel="Create account"
      title="sign up"
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
