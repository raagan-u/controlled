import { ReactNode, useMemo, useState } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import Animated, { FadeInDown } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ErrorBanner } from '@/components/ErrorBanner';
import { GoogleButton } from '@/components/GoogleButton';
import { PrimaryButton } from '@/components/PrimaryButton';
import { ShakeOnError, rise } from '@/motion';
import { colors, fonts } from '@/theme';

type Field = {
	  autoCapitalize?: 'none' | 'words';
	  autoComplete: 'email' | 'name' | 'password' | 'new-password';
  keyboardType?: 'email-address';
  label: string;
  name: string;
  secureTextEntry?: boolean;
};

export type AuthValues = Record<string, string>;
export type AuthErrors = Partial<Record<string, string>>;

type AuthFormScreenProps = {
  fields: Field[];
  footer: ReactNode;
  formError?: string;
  googleError?: string;
  googleLoading?: boolean;
  onGooglePress: () => void;
  onSubmit: (values: AuthValues) => void;
  submitting?: boolean;
  submitLabel: string;
  title: string;
  validate: (values: AuthValues) => AuthErrors;
};

export function AuthFormScreen({
  fields,
  footer,
  formError,
  googleError,
  googleLoading = false,
  onGooglePress,
  onSubmit,
  submitting = false,
  submitLabel,
  title,
  validate,
}: AuthFormScreenProps) {
  const [values, setValues] = useState<AuthValues>({});
  const [touched, setTouched] = useState<Set<string>>(new Set());
  const errors = useMemo(() => validate(values), [validate, values]);
  const isValid = Object.keys(errors).length === 0;

  const handleChange = (name: string, value: string) => {
    setValues((current) => ({ ...current, [name]: value }));
  };

  const handleSubmit = () => {
    setTouched(new Set(fields.map((field) => field.name)));
    if (isValid) {
      onSubmit(values);
    }
  };

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
          <Animated.Text entering={FadeInDown.duration(450)} style={styles.title}>{title}</Animated.Text>

          <View style={styles.fields}>
            {fields.map((field, index) => {
              const error = touched.has(field.name) ? errors[field.name] : undefined;

              return (
                <Animated.View entering={rise(index, 120)} key={field.name} style={styles.field}><ShakeOnError error={error}>
                  <TextInput
                    autoCapitalize={field.autoCapitalize ?? 'none'}
                    autoComplete={field.autoComplete}
                    autoCorrect={false}
                    keyboardType={field.keyboardType}
                    onBlur={() => setTouched((current) => new Set(current).add(field.name))}
                    onChangeText={(value) => handleChange(field.name, value)}
                    placeholder={field.label}
                    placeholderTextColor={colors.ink}
                    secureTextEntry={field.secureTextEntry}
                    style={[styles.input, error && styles.inputError]}
                    value={values[field.name] ?? ''}
                  />
                  </ShakeOnError>{error ? <Animated.Text entering={FadeInDown.duration(220)} style={styles.error}>{error}</Animated.Text> : null}
                </Animated.View>
              );
            })}
          </View>

          <PrimaryButton
            disabled={!isValid || submitting}
            label={submitLabel}
            onPress={handleSubmit}
          />
          {formError ? <ErrorBanner message={formError} /> : null}
          <View style={styles.footer}>{footer}</View>
          <Text style={styles.or}>or</Text>
          <GoogleButton disabled={googleLoading} onPress={onGooglePress} />
          {googleError ? <ErrorBanner message={googleError} /> : null}
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
    lineHeight: 72,
    marginBottom: 45,
    textTransform: 'lowercase',
  },
  fields: {
    gap: 28,
    marginBottom: 33,
    width: '100%',
  },
  field: {
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
  inputError: {
    borderColor: colors.error,
  },
  error: {
    color: colors.error,
    fontFamily: fonts.medium,
    fontSize: 13,
    lineHeight: 18,
    marginLeft: 20,
    marginTop: 5,
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
