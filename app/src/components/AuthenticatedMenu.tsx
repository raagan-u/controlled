import { useState } from 'react';
import { Modal, Pressable, StyleProp, StyleSheet, Text, View, ViewStyle } from 'react-native';
import { router, type Href, usePathname } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { getCurrentUser, signOut } from '@/api/wallet';
import type { User } from '@/api/types';
import { colors, fonts } from '@/theme';

const destinations: { label: string; route: Href }[] = [
  { label: 'Home', route: '/home' },
  { label: 'Requests', route: '/requests' },
  { label: 'Pacts', route: '/pacts' },
  { label: 'Accountability', route: '/accountability' },
  { label: 'Invitations', route: '/invitations' },
  { label: 'Activity', route: '/activity' },
];

export function AuthenticatedMenu({ initiallyOpen = false, style }: { initiallyOpen?: boolean; style?: StyleProp<ViewStyle> }) {
  const [open, setOpen] = useState(initiallyOpen);
  const [user, setUser] = useState<User | null>(null);
  const insets = useSafeAreaInsets();
  const pathname = usePathname();
  const show = () => {
    setOpen(true);
    if (!user) void getCurrentUser().then((value) => setUser(value.user)).catch(() => undefined);
  };
  const navigate = (route: Href) => {
    setOpen(false);
    if (route === '/home') router.replace(route); else router.push(route);
  };
  return <>
    <Pressable accessibilityLabel="Open menu" accessibilityRole="button" onPress={show} style={[styles.trigger, style]}><Text style={styles.menuIcon}>≡</Text></Pressable>
    <Modal animationType="fade" onRequestClose={() => setOpen(false)} transparent visible={open}>
      <Pressable onPress={() => setOpen(false)} style={styles.scrim}>
        <Pressable onPress={() => undefined} style={[styles.drawer, { paddingTop: Math.max(insets.top + 30, 58) }]}>
          <Pressable accessibilityLabel="Close menu" onPress={() => setOpen(false)} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable>
          <View style={styles.profile}><View style={styles.profileAvatar}><Text style={styles.profileInitial}>{user?.displayName.charAt(0) ?? 'C'}</Text></View><View style={styles.profileCopy}><Text numberOfLines={1} style={styles.drawerName}>{user?.displayName ?? 'ctrld'}</Text><Text style={styles.drawerID}>{user?.ctrldId ?? ''}</Text><Text numberOfLines={1} style={styles.drawerEmail}>{user?.email ?? ''}</Text></View></View>
          <Text style={styles.drawerKicker}>MENU —</Text>
          {destinations.map(({ label, route }) => { const active = route === '/home' ? pathname === '/home' : pathname.startsWith(String(route)); return <Pressable key={label} onPress={() => navigate(route)} style={[styles.drawerItem, active && styles.drawerItemActive]}><Text style={[styles.drawerItemText, active && styles.drawerItemTextActive]}>{label}</Text></Pressable>; })}
          <View style={styles.drawerBottom}><Text style={styles.drawerSmall}>Settings</Text><Text style={styles.drawerSmall}>Help</Text><Text onPress={() => void signOut().finally(() => router.replace('/welcome'))} style={styles.signOut}>Sign out</Text></View>
        </Pressable>
      </Pressable>
    </Modal>
  </>;
}

const styles = StyleSheet.create({
  trigger: { alignItems: 'center', backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: 12, height: 44, justifyContent: 'center', width: 44 },
  menuIcon: { color: colors.white, fontFamily: fonts.bold, fontSize: 27, lineHeight: 29 },
  scrim: { alignItems: 'flex-end', backgroundColor: colors.scrim, flex: 1 },
  drawer: { backgroundColor: colors.surfaceAlt, elevation: 18, flex: 1, paddingHorizontal: 24, shadowColor: colors.navy, shadowOffset: { height: 0, width: -5 }, shadowOpacity: 0.18, shadowRadius: 18, width: '76%' },
  close: { alignItems: 'center', alignSelf: 'flex-end', backgroundColor: colors.softButton, borderRadius: 10, height: 44, justifyContent: 'center', width: 44 },
  closeText: { color: colors.navy, fontFamily: fonts.medium, fontSize: 27 },
  profile: { alignItems: 'center', flexDirection: 'row', marginTop: 14 },
  profileAvatar: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 24, height: 48, justifyContent: 'center', marginRight: 12, width: 48 },
  profileInitial: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 },
  profileCopy: { flex: 1 },
  drawerKicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.7, marginBottom: 8, marginTop: 30 },
  drawerName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 17 },
  drawerID: { color: colors.blue, fontFamily: fonts.bold, fontSize: 12, marginTop: 2 },
  drawerEmail: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, marginTop: 2 },
  drawerItem: { borderRadius: 12, marginVertical: 2, paddingHorizontal: 12, paddingVertical: 11 },
  drawerItemActive: { backgroundColor: colors.blueTint },
  drawerItemText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 23 },
  drawerItemTextActive: { color: colors.blue },
  drawerBottom: { bottom: 40, left: 24, position: 'absolute' },
  drawerSmall: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 15, paddingVertical: 7 },
  signOut: { color: colors.redText, fontFamily: fonts.bold, fontSize: 15, marginTop: 9 },
});
