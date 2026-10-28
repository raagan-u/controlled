import { StyleSheet, Text, View } from 'react-native';

import { colors, fonts } from '@/theme';

export function Brand() {
  return (
    <View accessibilityLabel="Controlled" style={styles.row}>
      <Text style={styles.wordmark}>ctrld</Text>
      <View style={styles.period} />
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    alignItems: 'flex-end',
    flexDirection: 'row',
  },
  wordmark: {
    color: colors.ink,
    fontFamily: fonts.black,
    fontSize: 76,
    letterSpacing: -6,
    lineHeight: 86,
  },
  period: {
    backgroundColor: colors.ink,
    borderRadius: 7,
    height: 14,
    marginBottom: 15,
    marginLeft: 4,
    width: 14,
  },
});
