import { StyleSheet, Text } from 'react-native';

import { PressableScale } from '@/motion';
import { colors, fonts } from '@/theme';

type PrimaryButtonProps = {
  disabled?: boolean;
  label: string;
  onPress: () => void;
};

export function PrimaryButton({ disabled = false, label, onPress }: PrimaryButtonProps) {
  return (
    <PressableScale
      scaleTo={0.97}
      accessibilityState={{ disabled }}
      accessibilityRole="button"
      disabled={disabled}
      onPress={onPress}
      style={[styles.button, disabled && styles.disabled]}
    >
      <Text style={styles.label}>{label}</Text>
    </PressableScale>
  );
}

const styles = StyleSheet.create({
  button: {
    alignItems: 'center',
    backgroundColor: colors.primary,
    borderRadius: 26,
    height: 66,
    justifyContent: 'center',
    width: '100%',
  },
  disabled: {
    opacity: 0.5,
  },
  label: {
    color: colors.background,
    fontFamily: fonts.bold,
    fontSize: 21,
    lineHeight: 27,
  },
});
