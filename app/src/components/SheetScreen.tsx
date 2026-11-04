import { useCallback, useMemo, type PropsWithChildren, type ReactNode } from 'react';
import { BackHandler, KeyboardAvoidingView, PanResponder, Platform, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import Animated, { FadeInDown, interpolate, useAnimatedStyle, useSharedValue, withSpring, withTiming } from 'react-native-reanimated';
import { router, useFocusEffect, useLocalSearchParams, useNavigation, usePathname } from 'expo-router';
import { colors, radii } from '@/theme';
import { ScreenHeader } from './ScreenHeader';
type Props = PropsWithChildren<{ footer?: ReactNode; scroll?: boolean; title?: string }>;
// The money screens rise from the bottom like the Recent Activity sheet and can be swiped back down.
export const bottomSheetRoutes = ['/send', '/request-money', '/fund', '/withdraw'];
export function SheetScreen({ children, footer, scroll = false, title }: Props) {
  const { swap } = useLocalSearchParams<{ swap?: string }>();
  const navigation = useNavigation();
  const pathname = usePathname();
  const dismissible = bottomSheetRoutes.includes(pathname);
  const topLevel = pathname === '/requests';
  const { height } = useWindowDimensions();
  const ty = useSharedValue(0);
  const motion = useAnimatedStyle(() => ({ transform: [{ translateY: ty.value }] }));
  useFocusEffect(useCallback(() => {
    if (!topLevel) return;
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => { router.replace('/home'); return true; });
    return () => subscription.remove();
  }, [topLevel]));
  // The header fades as the card leaves so the Home screen underneath shows through.
  const headerMotion = useAnimatedStyle(() => ({ opacity: interpolate(ty.value, [0, height * 0.3], [1, 0], 'clamp') }));
  // Swipe down from anywhere on the screen drags only the white card (the blue header stays put);
  // past the threshold the card slides off, then the screen fades back to Home.
  const pan = useMemo(() => PanResponder.create({
    onMoveShouldSetPanResponderCapture: (_, gesture) => dismissible && gesture.dy > 6 && gesture.dy > Math.abs(gesture.dx) * 1.35,
    onPanResponderMove: (_, gesture) => ty.set(Math.max(0, gesture.dy - 6)),
    onPanResponderRelease: (_, gesture) => {
      if (gesture.dy > 110 || gesture.vy > 0.8) {
        ty.set(withTiming(height, { duration: 220 }));
        setTimeout(() => { navigation.setOptions({ animation: 'fade' }); if (router.canGoBack()) router.back(); else router.replace('/home'); }, 230);
      } else ty.set(withSpring(0, { damping: 28, mass: 0.9, overshootClamping: true, stiffness: 190 }));
    },
    onPanResponderTerminate: () => ty.set(withSpring(0, { damping: 28, mass: 0.9, overshootClamping: true, stiffness: 190 })),
    onPanResponderTerminationRequest: () => false,
  }), [dismissible, height, navigation, ty]);
  const content = scroll ? <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">{children}</ScrollView> : children;
  return <View {...pan.panHandlers} style={styles.outer}><KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={[styles.root, dismissible && styles.clear]}><Animated.View style={headerMotion}><ScreenHeader onBack={topLevel ? () => router.replace('/home') : undefined} title={title} /></Animated.View><Animated.View entering={swap === '1' || dismissible ? undefined : FadeInDown.duration(380).springify().damping(20).stiffness(150)} style={[styles.sheet, motion]}><View style={styles.grabber} />{content}{footer ? <View style={styles.footer}>{footer}</View> : null}</Animated.View></KeyboardAvoidingView></View>;
}
const styles = StyleSheet.create({ outer: { flex: 1 }, clear: { backgroundColor: 'transparent' }, root: { backgroundColor: colors.blue, flex: 1 }, sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, marginTop: -28, overflow: 'hidden', paddingHorizontal: 16, paddingTop: 18 }, grabber: { alignSelf: 'center', backgroundColor: colors.grabber, borderRadius: 3, height: 4, marginBottom: 18, width: 34 }, scroll: { flexGrow: 1, paddingBottom: 20 }, footer: { paddingBottom: 10, paddingTop: 8 } });
