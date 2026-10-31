import { StyleSheet, View } from 'react-native';

import { colors } from '@/theme';

// Soft layered circles behind a blue header; clip with overflow: 'hidden' on the parent.
export function HeroShapes() {
  return (
    <>
      <View pointerEvents="none" style={styles.deep} />
      <View pointerEvents="none" style={styles.soft} />
    </>
  );
}

const styles = StyleSheet.create({
  deep: { backgroundColor: colors.blueDeep, borderRadius: 160, height: 320, left: -120, opacity: 0.55, position: 'absolute', top: -90, width: 320 },
  soft: { backgroundColor: colors.blueSoft, borderRadius: 130, height: 260, opacity: 0.22, position: 'absolute', right: -90, top: 40, width: 260 },
});
