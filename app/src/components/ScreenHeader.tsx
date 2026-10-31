import { StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts } from '@/theme';
import { AuthenticatedMenu } from './AuthenticatedMenu';
import { HeroShapes } from './HeroShapes';
type Props = { title?: string };
export function ScreenHeader({ title = 'ctrld' }: Props) {
  const insets = useSafeAreaInsets();
  return <View style={[styles.header, { paddingTop: Math.max(insets.top, 14) }]}><HeroShapes /><View style={styles.side} /><Text style={styles.title}>{title}</Text><View style={[styles.side, styles.right]}><AuthenticatedMenu /></View></View>;
}
const styles = StyleSheet.create({ header: { alignItems: 'center', backgroundColor: colors.blue, flexDirection: 'row', minHeight: 160, overflow: 'hidden', paddingBottom: 42, paddingHorizontal: 16 }, side: { width: 50 }, right: { alignItems: 'flex-end' }, title: { color: colors.white, flex: 1, fontFamily: fonts.black, fontSize: 24, letterSpacing: -1.4, textAlign: 'center' } });
