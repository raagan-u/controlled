import { ReactNode } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleButton } from '@/components/GoogleButton';
import { PrimaryButton } from '@/components/PrimaryButton';
import { colors, fonts } from '@/theme';

type Field = {
  autoComplete: 'email' | 'password' | 'new-password';
  keyboardType?: 'email-address';
  label: string;
  secureTextEntry?: boolean;
};

type AuthFormScreenProps = {
  fields: Field[];
  footer: ReactNode;
  onGooglePress: () => void;
  onSubmit: () => void;
  submitLabel: string;
  title: string;
};

export function AuthFormScreen({
  fields,
  footer,
  onGooglePress,
  onSubmit,
  submitLabel,
  title,
}: AuthFormScreenProps) {
  return (
    <SafeAreaView edges={['top', 'bottom']} style={styles.safeArea}>
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={styles.flex}
      >
        <ScrollView
          contentContainerStyle={styles.content}
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}
        >
          <Text style={styles.title}>{title}</Text>

          <View style={styles.fields}>
            {fields.map((field) => (
              <TextInput
                autoCapitalize="none"
                autoComplete={field.autoComplete}
                keyboardType={field.keyboardType}
                key={field.label}
                placeholder={field.label}
                placeholderTextColor={colors.ink}
                secureTextEntry={field.secureTextEntry}
                style={styles.input}
              />
            ))}
          </View>

          <PrimaryButton label={submitLabel} onPress={onSubmit} />
          <View style={styles.footer}>{footer}</View>
          <Text style={styles.or}>or</Text>
          <GoogleButton onPress={onGooglePress} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  flex: {
    flex: 1,
  },
  safeArea: {
    backgroundColor: colors.background,
    flex: 1,
  },
  content: {
    alignItems: 'center',
    flexGrow: 1,
    paddingBottom: 32,
    paddingHorizontal: 18,
    paddingTop: 22,
  },
  title: {
    alignSelf: 'flex-start',
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 54,
    letterSpacing: -3.2,
    lineHeight: 62,
    marginBottom: 55,
    textTransform: 'lowercase',
  },
  fields: {
    gap: 28,
    marginBottom: 33,
    width: '100%',
  },
  input: {
    borderColor: colors.border,
    borderRadius: 26,
    borderWidth: 1.5,
    color: colors.ink,
    fontFamily: fonts.bold,
    fontSize: 17,
    height: 66,
    paddingHorizontal: 24,
    textAlign: 'center',
    width: '100%',
  },
  footer: {
    marginTop: 14,
  },
  or: {
    color: colors.muted,
    fontFamily: fonts.regular,
    fontSize: 17,
    lineHeight: 24,
    marginBottom: 28,
    marginTop: 10,
  },
});
