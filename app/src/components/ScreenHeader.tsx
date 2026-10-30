import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts } from '@/theme';
type Props = { back?: boolean; menu?: boolean; onMenu?: () => void; title?: string };
export function ScreenHeader({ back, menu, onMenu, title = 'ctrld' }: Props) {
  const insets = useSafeAreaInsets();
  return <View style={[styles.header, { paddingTop: Math.max(insets.top, 14) }]}><View style={styles.side}>{back ? <Pressable accessibilityLabel="Go back" onPress={() => router.back()} style={styles.icon}><Text style={styles.back}>‹</Text></Pressable> : null}</View><Text style={styles.title}>{title}</Text><View style={[styles.side, styles.right]}>{menu ? <Pressable accessibilityLabel="Open menu" onPress={onMenu} style={styles.icon}><Text style={styles.menu}>☰</Text></Pressable> : null}</View></View>;
}
const styles = StyleSheet.create({ header: { alignItems: 'center', backgroundColor: colors.blue, flexDirection: 'row', minHeight: 112, paddingBottom: 28, paddingHorizontal: 16 }, side: { width: 50 }, right: { alignItems: 'flex-end' }, title: { color: colors.white, flex: 1, fontFamily: fonts.black, fontSize: 24, letterSpacing: -1.4, textAlign: 'center' }, icon: { alignItems: 'center', height: 44, justifyContent: 'center', width: 44 }, back: { color: colors.white, fontFamily: fonts.medium, fontSize: 42, lineHeight: 42 }, menu: { color: colors.white, fontFamily: fonts.bold, fontSize: 26, lineHeight: 28 } });
