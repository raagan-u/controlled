import { useEffect } from 'react';
import { router } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { colors, fonts } from '@/theme';

export default function SplashScreen() {
  useEffect(() => {
    const timer = setTimeout(() => router.replace('/welcome'), 1200);
    return () => clearTimeout(timer);
  }, []);

  return (
    <View style={styles.container}>
      <Text accessibilityLabel="Controlled" style={styles.mark}>ctr_</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    backgroundColor: colors.background,
    flex: 1,
    justifyContent: 'center',
  },
  mark: {
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 64,
    letterSpacing: -5,
    lineHeight: 75,
  },
});
