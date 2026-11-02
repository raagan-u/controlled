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
  const [focusedField, setFocusedField] = useState<string | null>(null);
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
                  <Text style={[styles.fieldLabel, focusedField === field.name && styles.fieldLabelFocused]}>{field.label}</Text>
                  <TextInput
                    accessibilityLabel={field.label}
                    autoCapitalize={field.autoCapitalize ?? 'none'}
                    autoComplete={field.autoComplete}
                    autoCorrect={false}
                    keyboardType={field.keyboardType}
                    onBlur={() => {
                      setFocusedField(null);
                      setTouched((current) => new Set(current).add(field.name));
                    }}
                    onChangeText={(value) => handleChange(field.name, value)}
                    onFocus={() => setFocusedField(field.name)}
                    placeholder={field.name === 'confirmPassword' ? 'Re-enter your password' : `Enter your ${field.label}`}
                    placeholderTextColor={colors.textMuted}
                    secureTextEntry={field.secureTextEntry}
                    style={[styles.input, focusedField === field.name && styles.inputFocused, error && styles.inputError]}
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
          <View style={styles.orRow}><View style={styles.orLine} /><Text style={styles.or}>or</Text><View style={styles.orLine} /></View>
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
    gap: 20,
    marginBottom: 33,
    width: '100%',
  },
  field: {
    width: '100%',
  },
  input: {
    backgroundColor: colors.surface,
    borderColor: colors.divider,
    borderRadius: 14,
    borderWidth: 1,
    color: colors.ink,
    fontFamily: fonts.medium,
    fontSize: 16,
    height: 58,
    paddingHorizontal: 16,
    textAlign: 'left',
    width: '100%',
  },
  inputFocused: {
    backgroundColor: colors.white,
    borderColor: colors.blue,
    borderWidth: 1.5,
  },
  inputError: {
    borderColor: colors.error,
  },
  fieldLabel: {
    color: colors.textMuted,
    fontFamily: fonts.bold,
    fontSize: 10,
    letterSpacing: 1.3,
    marginBottom: 7,
    marginLeft: 3,
    textTransform: 'uppercase',
  },
  fieldLabelFocused: {
    color: colors.blue,
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
  orRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: 14,
    marginBottom: 28,
    marginTop: 10,
    width: '100%',
  },
  orLine: {
    backgroundColor: colors.grabber,
    flex: 1,
    height: 1.5,
  },
  or: {
    color: colors.muted,
    fontFamily: fonts.regular,
    fontSize: 17,
    lineHeight: 24,
  },
});
