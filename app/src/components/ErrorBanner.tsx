import { StyleSheet, Text, View } from 'react-native';
import Animated, { Easing, FadeOut, Keyframe } from 'react-native-reanimated';
import { ShakeOnError } from '@/motion';
import { colors, fonts } from '@/theme';

// A short, small rise-and-fade, followed by a subtle shake.
const appear = new Keyframe({ 0: { opacity: 0, transform: [{ translateY: 8 }] }, 100: { opacity: 1, transform: [{ translateY: 0 }], easing: Easing.out(Easing.cubic) } }).duration(260);

export function ErrorBanner({ message }: { message: string }) {
  return <Animated.View entering={appear} exiting={FadeOut.duration(150)} style={styles.wrap}>
    <ShakeOnError amplitude={4} delay={220} error={message} style={styles.box}>
      <View accessibilityElementsHidden style={styles.badge}><Text style={styles.bang}>!</Text></View>
      <Text accessibilityRole="alert" style={styles.text}>{message}</Text>
    </ShakeOnError>
  </Animated.View>;
}

const styles = StyleSheet.create({
  wrap: { marginTop: 14, width: '100%' },
  box: { alignItems: 'center', backgroundColor: colors.redTint, borderRadius: 20, flexDirection: 'row', gap: 12, paddingHorizontal: 16, paddingVertical: 14 },
  badge: { alignItems: 'center', backgroundColor: colors.red, borderRadius: 13, height: 26, justifyContent: 'center', width: 26 },
  bang: { color: colors.white, fontFamily: fonts.bold, fontSize: 15, lineHeight: 18 },
  text: { color: colors.redText, flex: 1, fontFamily: fonts.bold, fontSize: 14, lineHeight: 19 },
});
