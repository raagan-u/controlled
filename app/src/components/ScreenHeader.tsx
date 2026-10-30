import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts } from '@/theme';
type Props = { onMenu?: () => void; title?: string };
export function ScreenHeader({ onMenu, title = 'ctrld' }: Props) {
  const insets = useSafeAreaInsets();
  return <View style={[styles.header, { paddingTop: Math.max(insets.top, 14) }]}><View style={styles.side} /><Text style={styles.title}>{title}</Text><View style={[styles.side, styles.right]}><Pressable accessibilityLabel="Open menu" onPress={onMenu} style={styles.icon}><Text style={styles.menu}>≡</Text></Pressable></View></View>;
}
const styles = StyleSheet.create({ header: { alignItems: 'center', backgroundColor: colors.blue, flexDirection: 'row', minHeight: 160, paddingBottom: 42, paddingHorizontal: 16 }, side: { width: 50 }, right: { alignItems: 'flex-end' }, title: { color: colors.white, flex: 1, fontFamily: fonts.black, fontSize: 24, letterSpacing: -1.4, textAlign: 'center' }, icon: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: 12, height: 44, justifyContent: 'center', width: 44 }, menu: { color: colors.white, fontFamily: fonts.bold, fontSize: 28, lineHeight: 28 } });
