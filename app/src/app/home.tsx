import { useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { colors, fonts } from '@/theme';

export default function HomeScreen() {
  const { name } = useLocalSearchParams<{ name?: string | string[] }>();
  const displayName = Array.isArray(name) ? name[0] : name;

  return (
    <View style={styles.container}>
      <Text style={styles.greeting}>Hi, {displayName ?? ''}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    backgroundColor: colors.background,
    flex: 1,
    justifyContent: 'center',
    paddingHorizontal: 20,
  },
  greeting: {
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 54,
    letterSpacing: -3.2,
    lineHeight: 66,
    textAlign: 'center',
  },
});
