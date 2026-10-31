import type { PropsWithChildren, ReactNode } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, View } from 'react-native';
import Animated, { FadeInDown } from 'react-native-reanimated';
import { useLocalSearchParams } from 'expo-router';
import { colors, radii } from '@/theme';
import { ScreenHeader } from './ScreenHeader';
type Props = PropsWithChildren<{ footer?: ReactNode; scroll?: boolean; title?: string }>;
export function SheetScreen({ children, footer, scroll = false, title }: Props) {
  const { swap } = useLocalSearchParams<{ swap?: string }>();
  const content = scroll ? <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">{children}</ScrollView> : children;
  return <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={styles.root}><ScreenHeader title={title} /><Animated.View entering={swap === '1' ? undefined : FadeInDown.duration(380).springify().damping(20).stiffness(150)} style={styles.sheet}><View style={styles.grabber} />{content}{footer ? <View style={styles.footer}>{footer}</View> : null}</Animated.View></KeyboardAvoidingView>;
}
const styles = StyleSheet.create({ root: { backgroundColor: colors.blue, flex: 1 }, sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, marginTop: -28, overflow: 'hidden', paddingHorizontal: 16, paddingTop: 18 }, grabber: { alignSelf: 'center', backgroundColor: colors.grabber, borderRadius: 3, height: 4, marginBottom: 18, width: 34 }, scroll: { flexGrow: 1, paddingBottom: 20 }, footer: { paddingBottom: 10, paddingTop: 8 } });
