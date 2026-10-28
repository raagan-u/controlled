import { Image, Pressable, StyleSheet } from 'react-native';

const googleAsset = require('../../assets/auth/google-sign-in.png');

type GoogleButtonProps = {
  onPress: () => void;
};

export function GoogleButton({ onPress }: GoogleButtonProps) {
  return (
    <Pressable
      accessibilityLabel="Continue with Google"
      accessibilityRole="button"
      onPress={onPress}
      style={({ pressed }) => [styles.button, pressed && styles.pressed]}
    >
      <Image resizeMode="contain" source={googleAsset} style={styles.image} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    height: 44,
    width: 44,
  },
  image: {
    height: '100%',
    width: '100%',
  },
  pressed: {
    opacity: 0.72,
  },
});
