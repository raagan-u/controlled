import type { PropsWithChildren, ReactNode } from 'react';
import { router } from 'expo-router';
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts, radii } from '@/theme';

type Props = PropsWithChildren<{
  footer?: ReactNode;
  menu?: boolean;
  onMenu?: () => void;
  scroll?: boolean;
  title: string;
}>;

export function FeatureScreen({ children, footer, menu, onMenu, scroll = true, title }: Props) {
  const insets = useSafeAreaInsets();
  const body = scroll
    ? <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>{children}</ScrollView>
    : <View style={styles.body}>{children}</View>;
  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={styles.root}>
      <View style={[styles.header, { paddingTop: Math.max(insets.top + 8, 24) }]}>
        <Pressable accessibilityLabel="Go back" onPress={() => router.back()} style={styles.icon}><Text style={styles.back}>‹</Text></Pressable>
        <Text numberOfLines={1} style={styles.title}>{title}</Text>
        {menu ? <Pressable accessibilityLabel="Open menu" onPress={onMenu} style={styles.icon}><Text style={styles.menu}>≡</Text></Pressable> : <View style={styles.icon} />}
      </View>
      <View style={styles.sheet}>
        {body}
        {footer ? <View style={[styles.footer, { paddingBottom: Math.max(insets.bottom, 12) }]}>{footer}</View> : null}
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  root: { backgroundColor: colors.blue, flex: 1 },
  header: { alignItems: 'center', flexDirection: 'row', minHeight: 150, paddingBottom: 42, paddingHorizontal: 16 },
  icon: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: radii.md, height: 44, justifyContent: 'center', width: 44 },
  back: { color: colors.white, fontFamily: fonts.medium, fontSize: 35, lineHeight: 35, marginTop: -3 },
  menu: { color: colors.white, fontFamily: fonts.bold, fontSize: 27, lineHeight: 29 },
  title: { color: colors.white, flex: 1, fontFamily: fonts.bold, fontSize: 25, letterSpacing: -0.5, marginHorizontal: 10 },
  sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, marginTop: -28, overflow: 'hidden' },
  scroll: { flexGrow: 1, paddingBottom: 24, paddingHorizontal: 16, paddingTop: 22 },
  body: { flex: 1, paddingHorizontal: 16, paddingTop: 22 },
  footer: { backgroundColor: colors.white, paddingHorizontal: 16, paddingTop: 10 },
});
