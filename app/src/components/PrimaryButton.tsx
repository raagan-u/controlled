import { Pressable, StyleSheet, Text } from 'react-native';

import { colors, fonts } from '@/theme';

type PrimaryButtonProps = {
  label: string;
  onPress: () => void;
};

export function PrimaryButton({ label, onPress }: PrimaryButtonProps) {
  return (
    <Pressable
      accessibilityRole="button"
      onPress={onPress}
      style={({ pressed }) => [styles.button, pressed && styles.pressed]}
    >
      <Text style={styles.label}>{label}</Text>
    </Pressable>
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
  pressed: {
    opacity: 0.84,
    transform: [{ scale: 0.99 }],
  },
  label: {
    color: colors.background,
    fontFamily: fonts.bold,
    fontSize: 21,
    lineHeight: 27,
  },
});
