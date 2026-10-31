import { useEffect } from 'react';
import { router } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { getSessionToken } from '@/auth/session';
import { colors, fonts } from '@/theme';

export default function SplashScreen() {
  useEffect(() => {
    let active = true;
    const timer = setTimeout(() => {
      void getSessionToken().then((token) => {
        if (active) router.replace(token ? '/home' : '/welcome');
      });
    }, 800);
    return () => { active = false; clearTimeout(timer); };
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
