import { StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts } from '@/theme';
import { AuthenticatedMenu } from './AuthenticatedMenu';
import { HeroShapes } from './HeroShapes';
type Props = { title?: string };
export function ScreenHeader({ title = 'ctrld' }: Props) {
  const insets = useSafeAreaInsets();
  // Mirrors the Home hero (same top padding, 48px row, 32px wordmark) so the logo doesn't jump or resize
  // when a screen slides over Home.
  return <View style={[styles.header, { paddingTop: Math.max(insets.top + 10, 28) }]}><HeroShapes /><View style={styles.row}><View style={styles.side} /><Text style={styles.title}>{title}</Text><View style={[styles.side, styles.right]}><AuthenticatedMenu /></View></View></View>;
}
const styles = StyleSheet.create({ header: { backgroundColor: colors.blue, minHeight: 160, overflow: 'hidden', paddingBottom: 42, paddingHorizontal: 16 }, row: { alignItems: 'center', flexDirection: 'row', height: 48 }, side: { height: 48, justifyContent: 'center', width: 48 }, right: { alignItems: 'flex-end' }, title: { color: colors.white, flex: 1, fontFamily: fonts.black, fontSize: 32, letterSpacing: -2, textAlign: 'center' } });
