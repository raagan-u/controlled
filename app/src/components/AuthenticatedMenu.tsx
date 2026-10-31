import { useEffect, useState } from 'react';
import { Modal, Pressable, StyleProp, StyleSheet, Text, useWindowDimensions, View, ViewStyle } from 'react-native';
import Animated, { Easing, FadeIn, FadeInRight, interpolate, runOnJS, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { PressableScale } from '@/motion';
import { router, type Href, usePathname } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { getCurrentUser, signOut } from '@/api/wallet';
import type { User } from '@/api/types';
import { colors, fonts } from '@/theme';

// Distinct value per request so re-selecting Activity on Home re-triggers the expand.
let activityRequests = 0;

const destinations: { label: string; opensActivity?: boolean; route: Href }[] = [
  { label: 'Home', route: '/home' },
  { label: 'Requests', route: '/requests' },
  { label: 'Pacts', route: '/pacts' },
  { label: 'Accountability', route: '/accountability' },
  { label: 'Invitations', route: '/invitations' },
  { label: 'Activity', opensActivity: true, route: '/home' },
];

export function AuthenticatedMenu({ initiallyOpen = false, style }: { initiallyOpen?: boolean; style?: StyleProp<ViewStyle> }) {
  const [open, setOpen] = useState(initiallyOpen);
  const [user, setUser] = useState<User | null>(null);
  const [copied, setCopied] = useState(false);
  const insets = useSafeAreaInsets();
  const { width } = useWindowDimensions();
  const progress = useSharedValue(0);
  const pathname = usePathname();
  useEffect(() => { if (open) progress.set(withTiming(1, { duration: 320, easing: Easing.out(Easing.cubic) })); }, [open, progress]);
  const hide = () => { progress.set(withTiming(0, { duration: 220, easing: Easing.in(Easing.cubic) }, (finished) => { if (finished) runOnJS(setOpen)(false); })); };
  const scrimStyle = useAnimatedStyle(() => ({ opacity: progress.value }));
  const drawerStyle = useAnimatedStyle(() => ({ transform: [{ translateX: interpolate(progress.value, [0, 1], [width * 0.76, 0]) }] }));
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 1600);
    return () => clearTimeout(timer);
  }, [copied]);
  // Loaded on demand so a build without the native clipboard module can't crash the app at startup.
  const copyCtrldId = async () => {
    if (!user?.ctrldId) return;
    try {
      const { setStringAsync } = await import('expo-clipboard');
      await setStringAsync(user.ctrldId);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  const show = () => {
    setOpen(true);
    if (!user) void getCurrentUser().then((value) => setUser(value.user)).catch(() => undefined);
  };
  const navigate = (route: Href, opensActivity?: boolean) => {
    progress.set(0);
    setOpen(false);
    // Activity is Home's expanded panel; the changing param re-triggers it even when already on Home.
    if (opensActivity) router.replace({ pathname: '/home', params: { openActivity: String(++activityRequests) } });
    else if (route === '/home') router.replace(route); else router.push(route);
  };
  return <>
    <PressableScale accessibilityLabel="Open menu" accessibilityRole="button" onPress={show} scaleTo={0.9} style={[styles.trigger, style]}><Text style={styles.menuIcon}>≡</Text></PressableScale>
    <Modal animationType="none" onRequestClose={hide} transparent visible={open}>
      <View style={styles.modalRoot}>
        <Animated.View pointerEvents="none" style={[styles.scrimFill, scrimStyle]} />
        <Pressable onPress={hide} style={styles.scrim}>
        <Animated.View style={[styles.drawerWrap, drawerStyle]}><Pressable onPress={() => undefined} style={[styles.drawer, { paddingTop: Math.max(insets.top + 30, 58) }]}>
          <Pressable accessibilityLabel="Close menu" onPress={hide} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable>
          <View style={styles.profile}><View style={styles.profileAvatar}><Text style={styles.profileInitial}>{user?.displayName.charAt(0) ?? 'C'}</Text></View><View style={styles.profileCopy}><Text numberOfLines={1} style={styles.drawerName}>{user?.displayName ?? 'ctrld'}</Text><PressableScale accessibilityHint="Copies your ctrld ID" accessibilityLabel={`Copy ctrld ID ${user?.ctrldId ?? ''}`} accessibilityRole="button" hitSlop={8} onPress={() => void copyCtrldId()} scaleTo={0.94} style={styles.idButton}>{copied ? <Animated.Text entering={FadeIn.duration(150)} key="copied" style={[styles.drawerID, styles.copied]}>Copied ✓</Animated.Text> : <Text key="id" style={styles.drawerID}>{user?.ctrldId ?? ''}</Text>}</PressableScale><Text numberOfLines={1} style={styles.drawerEmail}>{user?.email ?? ''}</Text></View></View>
          <Text style={styles.drawerKicker}>MENU —</Text>
          {destinations.map(({ label, opensActivity, route }, index) => { const active = opensActivity ? false : route === '/home' ? pathname === '/home' : pathname.startsWith(String(route)); return <PressableScale entering={FadeInRight.delay(120 + index * 45).duration(320)} key={label} onPress={() => navigate(route, opensActivity)} scaleTo={0.97} style={[styles.drawerItem, active && styles.drawerItemActive]}><Text style={[styles.drawerItemText, active && styles.drawerItemTextActive]}>{label}</Text></PressableScale>; })}
          <View style={styles.drawerBottom}><Text style={styles.drawerSmall}>Settings</Text><Text style={styles.drawerSmall}>Help</Text><Text onPress={() => void signOut().finally(() => router.replace('/welcome'))} style={styles.signOut}>Sign out</Text></View>
        </Pressable></Animated.View>
        </Pressable>
      </View>
    </Modal>
  </>;
}

const styles = StyleSheet.create({
  trigger: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: 12, height: 44, justifyContent: 'center', width: 44 },
  menuIcon: { color: colors.white, fontFamily: fonts.bold, fontSize: 27, lineHeight: 29 },
  modalRoot: { flex: 1 },
  scrimFill: { backgroundColor: colors.scrim, ...StyleSheet.absoluteFill },
  scrim: { alignItems: 'flex-end', flex: 1 },
  drawerWrap: { height: '100%', width: '76%' },
  drawer: { backgroundColor: colors.surfaceAlt, elevation: 18, flex: 1, paddingHorizontal: 24, shadowColor: colors.navy, shadowOffset: { height: 0, width: -5 }, shadowOpacity: 0.18, shadowRadius: 18 },
  close: { alignItems: 'center', alignSelf: 'flex-end', backgroundColor: colors.softButton, borderRadius: 10, height: 44, justifyContent: 'center', width: 44 },
  closeText: { color: colors.navy, fontFamily: fonts.medium, fontSize: 27 },
  profile: { alignItems: 'center', flexDirection: 'row', marginTop: 18 },
  profileAvatar: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 32, height: 64, justifyContent: 'center', marginRight: 14, width: 64 },
  profileInitial: { color: colors.white, fontFamily: fonts.bold, fontSize: 26 },
  profileCopy: { flex: 1 },
  drawerKicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.7, marginBottom: 8, marginTop: 34 },
  drawerName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 22, letterSpacing: -0.4 },
  idButton: { alignSelf: 'flex-start' },
  copied: { color: colors.greenText },
  drawerID: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14, marginTop: 3 },
  drawerEmail: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 3 },
  drawerItem: { borderRadius: 12, marginVertical: 2, paddingHorizontal: 12, paddingVertical: 11 },
  drawerItemActive: { backgroundColor: colors.blueTint },
  drawerItemText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25 },
  drawerItemTextActive: { color: colors.blue },
  drawerBottom: { bottom: 40, left: 24, position: 'absolute' },
  drawerSmall: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 17, paddingVertical: 7 },
  signOut: { color: colors.redText, fontFamily: fonts.bold, fontSize: 17, marginTop: 9 },
});
