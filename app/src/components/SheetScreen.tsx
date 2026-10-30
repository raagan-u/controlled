import type { PropsWithChildren, ReactNode } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, View } from 'react-native';
import { colors, radii } from '@/theme';
import { ScreenHeader } from './ScreenHeader';
type Props = PropsWithChildren<{ back?: boolean; footer?: ReactNode; scroll?: boolean; title?: string }>;
export function SheetScreen({ back = true, children, footer, scroll = false, title }: Props) {
  const content = scroll ? <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">{children}</ScrollView> : children;
  return <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={styles.root}><ScreenHeader back={back} title={title} /><View style={styles.sheet}>{content}{footer ? <View style={styles.footer}>{footer}</View> : null}</View></KeyboardAvoidingView>;
}
const styles = StyleSheet.create({ root: { backgroundColor: colors.blue, flex: 1 }, sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, marginTop: -28, overflow: 'hidden', paddingHorizontal: 18, paddingTop: 26 }, scroll: { flexGrow: 1, paddingBottom: 20 }, footer: { paddingBottom: 12, paddingTop: 12 } });
