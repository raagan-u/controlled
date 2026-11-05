import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts } from '@/theme';
import { AuthenticatedMenu } from './AuthenticatedMenu';
import { HeroShapes } from './HeroShapes';
type Props = { onBack?: () => void; onTitlePress?: () => void; title?: string };
export function ScreenHeader({ onBack, onTitlePress, title = 'ctrld' }: Props) {
  const insets = useSafeAreaInsets();
  // Mirrors the Home hero (same top padding, 48px row, 32px wordmark) so the logo doesn't jump or resize
  // when a screen slides over Home.
  return <View style={[styles.header, { paddingTop: Math.max(insets.top + 10, 28) }]}><HeroShapes /><View style={styles.row}>{onBack ? <Pressable accessibilityLabel="Go home" onPress={onBack} style={[styles.side, styles.backButton]}><Text style={styles.back}>‹</Text></Pressable> : <View style={styles.side} />}<Pressable accessibilityLabel={onTitlePress ? 'Go home' : undefined} accessibilityRole={onTitlePress ? 'button' : undefined} disabled={!onTitlePress} onPress={onTitlePress} style={styles.titleButton}><Text style={styles.title}>{title}</Text></Pressable><View style={[styles.side, styles.right]}><AuthenticatedMenu /></View></View></View>;
}
const styles = StyleSheet.create({ header: { backgroundColor: colors.blue, minHeight: 160, overflow: 'hidden', paddingBottom: 42, paddingHorizontal: 16 }, row: { alignItems: 'center', flexDirection: 'row', height: 48 }, side: { height: 48, justifyContent: 'center', width: 48 }, backButton: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: 12, height: 44, width: 44 }, back: { color: colors.white, fontFamily: fonts.medium, fontSize: 35, lineHeight: 35, marginTop: -3 }, right: { alignItems: 'flex-end' }, titleButton: { flex: 1, justifyContent: 'center', minHeight: 44 }, title: { color: colors.white, fontFamily: fonts.black, fontSize: 32, letterSpacing: -2, textAlign: 'center' } });
