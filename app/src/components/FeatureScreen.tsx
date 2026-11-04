import { useCallback, useEffect, useRef, type PropsWithChildren, type ReactNode } from 'react';
import { router, useFocusEffect, usePathname } from 'expo-router';
import { BackHandler, Keyboard, KeyboardAvoidingView, Platform, Pressable, ScrollView, type ScrollViewInstance, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts, radii } from '@/theme';
import { AuthenticatedMenu } from './AuthenticatedMenu';
import { HeroShapes } from './HeroShapes';

type Props = PropsWithChildren<{
  footer?: ReactNode;
  onBack?: () => void;
  scroll?: boolean;
	scrollToEnd?: boolean;
  title: string;
}>;

export function FeatureScreen({ children, footer, onBack, scroll = true, scrollToEnd = false, title }: Props) {
  const insets = useSafeAreaInsets();
	const pathname = usePathname();
	const topLevel = ['/pacts', '/pacts/recovery', '/requests', '/accountability', '/invitations'].includes(pathname);
	const scrollRef = useRef<ScrollViewInstance>(null);
	const goBack = onBack ?? (() => {
		if (topLevel) {
			router.replace('/home');
			return;
		}
		if (router.canGoBack()) router.back();
		else router.replace('/home');
	});
	useFocusEffect(useCallback(() => {
		if (!topLevel) return;
		const subscription = BackHandler.addEventListener('hardwareBackPress', () => { router.replace('/home'); return true; });
		return () => subscription.remove();
	}, [topLevel]));
	useEffect(() => {
		if (!scrollToEnd) return;
		const revealBottom = () => requestAnimationFrame(() => scrollRef.current?.scrollToEnd({ animated: true }));
		revealBottom();
		const subscription = Keyboard.addListener('keyboardDidShow', revealBottom);
		return () => subscription.remove();
	}, [scrollToEnd]);
  const body = scroll
    ? <ScrollView automaticallyAdjustKeyboardInsets contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled" ref={scrollRef} showsVerticalScrollIndicator={false}>{children}</ScrollView>
    : <View style={styles.body}>{children}</View>;
  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : 'height'} style={styles.root}>
      <View style={[styles.header, { paddingTop: Math.max(insets.top + 8, 24) }]}>
        <HeroShapes />
        <Pressable accessibilityLabel="Go back" onPress={goBack} style={styles.icon}><Text style={styles.back}>‹</Text></Pressable>
        <Text numberOfLines={1} style={styles.title}>{title}</Text>
        <AuthenticatedMenu style={styles.icon} />
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
  header: { alignItems: 'center', flexDirection: 'row', minHeight: 150, overflow: 'hidden', paddingBottom: 42, paddingHorizontal: 16 },
  icon: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: radii.md, height: 44, justifyContent: 'center', width: 44 },
  back: { color: colors.white, fontFamily: fonts.medium, fontSize: 35, lineHeight: 35, marginTop: -3 },
  title: { color: colors.white, flex: 1, fontFamily: fonts.bold, fontSize: 25, letterSpacing: -0.5, marginHorizontal: 10 },
  sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, marginTop: -28, overflow: 'hidden' },
  scroll: { flexGrow: 1, paddingBottom: 24, paddingHorizontal: 16, paddingTop: 22 },
  body: { flex: 1, paddingHorizontal: 16, paddingTop: 22 },
  footer: { backgroundColor: colors.white, paddingHorizontal: 16, paddingTop: 10 },
});
