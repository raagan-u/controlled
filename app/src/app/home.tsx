import { router, type Href, useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import { ActivityIndicator, Modal, PanResponder, Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ActivityRow } from '@/components/ActivityRow';
import { formatUSD, signOut } from '@/api/wallet';
import { transactionToActivity } from '@/data/activity';
import { useWalletOverview } from '@/hooks/useWalletOverview';
import { colors, fonts, radii } from '@/theme';

export default function HomeScreen() {
  const { name } = useLocalSearchParams<{ name?: string | string[] }>();
  const { data, error, loading, refresh } = useWalletOverview();
  const displayName = data?.user.displayName || (Array.isArray(name) ? name[0] : name) || 'Alex';
  const balance = data ? formatUSD(data.wallet.balances.available.amountMinor) : '—';
  const activities = useMemo(() => data ? data.transactions.map((item) => transactionToActivity(item, data.wallet)) : [], [data]);
  const recentUsers = useMemo(() => {
    if (!data) return [];
    const seen = new Set<string>();
    return [...data.transactions]
      .sort((left, right) => new Date(right.occurredAt).getTime() - new Date(left.occurredAt).getTime())
      .flatMap((transaction) => {
        const transfer = transaction.transferDetails;
        if (!transfer) return [];
        return [transfer.sender.id === data.user.id ? transfer.recipient : transfer.sender];
      })
      .filter((user) => {
        if (seen.has(user.id)) return false;
        seen.add(user.id);
        return true;
      })
      .slice(0, 4);
  }, [data]);
  const [menu, setMenu] = useState(false);
  const [dashboardPan] = useState(() => PanResponder.create({
    onMoveShouldSetPanResponder: (_, gesture) =>
      gesture.dy < -12 && Math.abs(gesture.dy) > Math.abs(gesture.dx),
    onPanResponderRelease: (_, gesture) => {
      if (gesture.dy < -40) router.push('/activity' as Href);
    },
  }));
  const insets = useSafeAreaInsets();
  return <View style={styles.root}>
    <View style={[styles.hero, { paddingTop: Math.max(insets.top + 10, 28) }]}>
      <View style={styles.heroTop}><View style={styles.menuButton} /><Text style={styles.brand}>ctrld</Text><Pressable accessibilityLabel="Open menu" onPress={() => setMenu(true)} style={styles.menuButton}><Text style={styles.menuIcon}>≡</Text></Pressable></View>
      <View style={styles.balanceWrap}><Text style={styles.available}>AVAILABLE</Text><Text adjustsFontSizeToFit numberOfLines={1} style={styles.balance}>${balance}</Text></View>
    </View>
    <View style={styles.content}>
      <View style={styles.actions}><Pressable onPress={() => router.push('/fund' as Href)} style={styles.addAction}><Text style={styles.plus}>＋</Text><Text style={styles.addText}>Add money</Text></Pressable><Pressable accessibilityLabel="Send or request money" onPress={() => router.push('/send' as Href)} style={styles.centerAction}><View style={styles.centerIcon}><Text style={styles.centerArrow}>↑</Text><Text style={styles.centerArrow}>↓</Text></View></Pressable><Pressable onPress={() => router.push('/withdraw' as Href)} style={styles.action}><Text style={styles.actionText}>Withdraw</Text></Pressable></View>
      <View {...dashboardPan.panHandlers} style={styles.panel}>
        <View style={styles.dashboardRow}><View style={styles.recentsColumn}><Text style={styles.kicker}>RECENTS —</Text><View style={styles.recentsGrid}>{Array.from({ length: 4 }, (_, index) => recentUsers[index]).map((user, index) => user ? <Pressable accessibilityLabel={`Send money to ${user.displayName}`} key={user.id} onPress={() => router.push(`/send?recipient=${encodeURIComponent(user.ctrldId)}` as Href)} style={styles.person}><View style={styles.avatar}><Text style={styles.avatarText}>{user.displayName.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()}</Text></View></Pressable> : <View key={`empty-${index}`} style={styles.person}><View style={[styles.avatar, styles.avatarEmpty]} /></View>)}</View></View><View style={styles.pactsColumn}><Text style={styles.kicker}>ACTIVE PACTS —</Text><View style={styles.pactCard}><Text style={styles.pactName}>No active pacts</Text><Text style={styles.pactEmpty}>Your active spending limits will appear here.</Text></View></View></View>
        <Pressable onPress={() => router.push('/activity' as Href)} style={styles.activityHeading}><Text style={styles.kicker}>RECENT ACTIVITY —</Text><Text style={styles.activityArrow}>→</Text></Pressable>
        <View style={styles.activityViewport}>{loading && !data ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : error && !data ? <Pressable onPress={() => void refresh()} style={styles.state}><Text style={styles.stateTitle}>Couldn’t load wallet</Text><Text style={styles.stateCopy}>Tap to try again</Text></Pressable> : activities.length === 0 ? <View style={styles.state}><Text style={styles.stateTitle}>No activity yet</Text><Text style={styles.stateCopy}>Your wallet transactions will appear here.</Text></View> : activities.slice(0, 3).map((item) => <ActivityRow card item={item} key={item.id} />)}</View>
      </View>
    </View>
    <Modal animationType="fade" onRequestClose={() => setMenu(false)} transparent visible={menu}><Pressable onPress={() => setMenu(false)} style={styles.scrim}><Pressable onPress={() => undefined} style={[styles.drawer, { paddingTop: Math.max(insets.top + 30, 58) }]}><Pressable onPress={() => setMenu(false)} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable><View style={styles.profile}><View style={styles.profileAvatar}><Text style={styles.profileInitial}>{displayName.charAt(0)}</Text></View><View><Text style={styles.drawerName}>{displayName}</Text><Text style={styles.drawerID}>{data?.user.ctrldId ?? ''}</Text><Text style={styles.drawerEmail}>{data?.user.email ?? ''}</Text></View></View><Text style={styles.drawerKicker}>MENU —</Text>{['Home', 'Requests', 'Pacts', 'Accountability', 'Activity'].map((item) => <Pressable key={item} onPress={() => { setMenu(false); if (item === 'Activity') router.push('/activity' as Href); if (item === 'Requests') router.push('/requests' as Href); }} style={[styles.drawerItem, item === 'Home' && styles.drawerItemActive]}><Text style={[styles.drawerItemText, item === 'Home' && styles.drawerItemTextActive]}>{item}</Text>{item === 'Accountability' ? <Text style={styles.badge}>2</Text> : null}</Pressable>)}<View style={styles.drawerBottom}><Text style={styles.drawerSmall}>Settings</Text><Text style={styles.drawerSmall}>Help</Text><Text onPress={() => void signOut().finally(() => router.replace('/welcome'))} style={styles.signOut}>Sign out</Text></View></Pressable></Pressable></Modal>
  </View>;
}
const styles = StyleSheet.create({
  root: { backgroundColor: colors.white, flex: 1 }, hero: { backgroundColor: colors.blue, borderBottomLeftRadius: radii.sheet, borderBottomRightRadius: radii.sheet, height: 294, overflow: 'hidden', paddingHorizontal: 16, zIndex: 2 }, heroTop: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' }, brand: { color: colors.white, fontFamily: fonts.black, fontSize: 25, letterSpacing: -1.5 }, menuButton: { alignItems: 'center', height: 48, justifyContent: 'center', width: 48 }, menuIcon: { backgroundColor: 'rgba(255,255,255,0.14)', borderRadius: 12, color: colors.white, fontFamily: fonts.bold, fontSize: 27, height: 44, lineHeight: 38, overflow: 'hidden', textAlign: 'center', width: 44 }, balanceWrap: { alignItems: 'center', flex: 1, justifyContent: 'center', paddingBottom: 18 }, available: { color: colors.onBlueMuted, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.7 }, balance: { color: colors.white, fontFamily: fonts.bold, fontSize: 44, letterSpacing: -1.5, marginTop: 6 },
  content: { backgroundColor: colors.white, flex: 1, gap: 16, paddingTop: 16 }, actions: { backgroundColor: colors.white, flexDirection: 'row', gap: 10, paddingHorizontal: 14 }, action: { alignItems: 'center', backgroundColor: colors.softButton, borderRadius: 12, flex: 1, height: 56, justifyContent: 'center' }, actionText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16 }, addAction: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 12, flex: 1, flexDirection: 'row', height: 56, justifyContent: 'center' }, plus: { color: colors.white, fontFamily: fonts.medium, fontSize: 24, marginRight: 4 }, addText: { color: colors.white, fontFamily: fonts.bold, fontSize: 15 }, centerAction: { alignItems: 'center', backgroundColor: colors.navy, borderRadius: 12, height: 56, justifyContent: 'center', width: 56 }, centerIcon: { alignItems: 'center', flexDirection: 'row', gap: 3, justifyContent: 'center' }, centerArrow: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 }, panel: { backgroundColor: colors.surface, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, paddingBottom: 40, paddingHorizontal: 14, paddingTop: 20 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 }, dashboardRow: { flexDirection: 'row', gap: 10 }, recentsColumn: { flex: 1 }, pactsColumn: { flex: 1.4 }, recentsGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 10 }, person: { width: '45%' }, avatar: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: 12, height: 64, justifyContent: 'center', width: '100%' }, avatarEmpty: { backgroundColor: colors.softButton }, avatarText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14 }, pactCard: { backgroundColor: colors.white, borderRadius: 14, height: 136, marginTop: 10, padding: 14 }, pactName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 14 }, pactEmpty: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, lineHeight: 17, marginTop: 12 },
  activityHeading: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginTop: 24, paddingHorizontal: 2, paddingVertical: 3 }, activityArrow: { color: colors.navy, fontFamily: fonts.medium, fontSize: 22 }, activityViewport: { height: 164, marginTop: 10, overflow: 'hidden' },
  loader: { marginTop: 42 }, state: { alignItems: 'center', backgroundColor: colors.white, borderRadius: 16, justifyContent: 'center', minHeight: 100, padding: 18 }, stateTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, stateCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 4, textAlign: 'center' },
  scrim: { alignItems: 'flex-end', backgroundColor: colors.scrim, flex: 1 }, drawer: { backgroundColor: colors.surfaceAlt, elevation: 18, flex: 1, paddingHorizontal: 24, shadowColor: colors.navy, shadowOffset: { height: 0, width: -5 }, shadowOpacity: 0.18, shadowRadius: 18, width: '76%' }, close: { alignItems: 'center', alignSelf: 'flex-end', backgroundColor: colors.softButton, borderRadius: 10, height: 44, justifyContent: 'center', width: 44 }, closeText: { color: colors.navy, fontFamily: fonts.medium, fontSize: 27 }, profile: { alignItems: 'center', flexDirection: 'row', marginTop: 14 }, profileAvatar: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 24, height: 48, justifyContent: 'center', marginRight: 12, width: 48 }, profileInitial: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 }, drawerKicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.7, marginBottom: 8, marginTop: 30 }, drawerName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 17 }, drawerID: { color: colors.blue, fontFamily: fonts.bold, fontSize: 12, marginTop: 2 }, drawerEmail: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, marginTop: 2 }, drawerItem: { alignItems: 'center', borderRadius: 12, flexDirection: 'row', justifyContent: 'space-between', marginVertical: 2, paddingHorizontal: 12, paddingVertical: 11 }, drawerItemActive: { backgroundColor: colors.blueTint }, drawerItemText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 23 }, drawerItemTextActive: { color: colors.blue }, badge: { backgroundColor: colors.red, borderRadius: 10, color: colors.white, fontFamily: fonts.bold, fontSize: 10, overflow: 'hidden', paddingHorizontal: 7, paddingVertical: 4 }, drawerBottom: { bottom: 40, left: 24, position: 'absolute' }, drawerSmall: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 15, paddingVertical: 7 }, signOut: { color: colors.redText, fontFamily: fonts.bold, fontSize: 15, marginTop: 9 },
});
