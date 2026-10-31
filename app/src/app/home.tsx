import { router, type Href, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useMemo, useState } from 'react';
import { ActivityIndicator, PanResponder, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ActivityRow } from '@/components/ActivityRow';
import { AuthenticatedMenu } from '@/components/AuthenticatedMenu';
import { formatUSD } from '@/api/wallet';
import { transactionToActivity, visibleTransactions } from '@/data/activity';
import { useWalletOverview } from '@/hooks/useWalletOverview';
import { colors, fonts, radii } from '@/theme';
import { getPacts } from '@/api/pacts';
import type { Pact } from '@/api/types';

export default function HomeScreen() {
  const { openMenu } = useLocalSearchParams<{ openMenu?: string }>();
  const { data, error, loading, refresh } = useWalletOverview();
  const balance = data ? formatUSD(data.wallet.balances.available.amountMinor) : '—';
  const activities = useMemo(() => data ? visibleTransactions(data.transactions).map((item) => transactionToActivity(item, data.wallet)) : [], [data]);
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
  const [pacts, setPacts] = useState<Pact[]>([]);
  const [activityViewportHeight, setActivityViewportHeight] = useState(0);
  const activityCardHeight = activityViewportHeight
    ? Math.max(68, Math.min(94, (activityViewportHeight - 16) / 2.25))
    : undefined;
  useFocusEffect(useCallback(() => { void getPacts().then((items) => setPacts(items.filter((pact) => pact.status === 'ACTIVE' || pact.status === 'SCHEDULED'))).catch(() => setPacts([])); }, []));
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
      <View style={styles.heroTop}><View style={styles.menuButton} /><Text style={styles.brand}>ctrld</Text><View style={styles.menuButton}><AuthenticatedMenu initiallyOpen={openMenu === '1'} /></View></View>
      <View style={styles.balanceWrap}><Text style={styles.available}>AVAILABLE</Text><Text adjustsFontSizeToFit numberOfLines={1} style={styles.balance}>${balance}</Text></View>
    </View>
    <View style={styles.content}>
      <View style={styles.actions}><Pressable onPress={() => router.push('/fund' as Href)} style={styles.addAction}><Text style={styles.plus}>＋</Text><Text style={styles.addText}>Add money</Text></Pressable><Pressable accessibilityLabel="Send or request money" onPress={() => router.push('/send' as Href)} style={styles.centerAction}><View style={styles.centerIcon}><Text style={styles.centerArrow}>↑</Text><Text style={styles.centerArrow}>↓</Text></View></Pressable><Pressable onPress={() => router.push('/withdraw' as Href)} style={styles.action}><Text style={styles.actionText}>Withdraw</Text></Pressable></View>
      <View {...dashboardPan.panHandlers} style={[styles.panel, { paddingBottom: Math.max(insets.bottom + 6, 14) }]}>
        <View style={styles.dashboardRow}><View style={styles.recentsColumn}><Text style={styles.kicker}>RECENTS —</Text><View style={styles.recentsGrid}>{Array.from({ length: 4 }, (_, index) => recentUsers[index]).map((user, index) => user ? <Pressable accessibilityLabel={`Send money to ${user.displayName}`} key={user.id} onPress={() => router.push(`/send?recipient=${encodeURIComponent(user.ctrldId)}` as Href)} style={styles.person}><View style={styles.avatar}><Text style={styles.avatarText}>{user.displayName.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()}</Text></View></Pressable> : <View key={`empty-${index}`} style={styles.person}><View style={[styles.avatar, styles.avatarEmpty]} /></View>)}</View></View><View style={styles.pactsColumn}><Text style={styles.kicker}>ACTIVE PACTS —</Text>{pacts.length ? <ScrollView horizontal pagingEnabled showsHorizontalScrollIndicator={false} style={styles.pactCarousel}>{pacts.map((pact) => { const available = pact.progressAvailable !== false; const ratio = available ? pact.progress.usedMinor / pact.terms.spendingLimit.amountMinor : 0; const backgroundColor = !available ? colors.surface : pact.progress.overageMinor ? colors.redTint : ratio >= 0.8 ? colors.orangeTint : ratio < 0.5 ? colors.greenTint : colors.blueTint; return <Pressable key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} style={[styles.pactCard, { backgroundColor }]}><Text numberOfLines={2} style={styles.pactName}>{pact.terms.goal}</Text>{available ? <><Text style={styles.pactAmount}>${formatUSD(pact.progress.usedMinor)} <Text style={styles.pactOf}>of ${formatUSD(pact.terms.spendingLimit.amountMinor)}</Text></Text><View style={styles.pactTrack}><View style={[styles.pactFill, { width: `${Math.max(4, Math.min(100, ratio * 100))}%` }]} /></View><Text style={styles.pactEmpty}>${formatUSD(pact.progress.remainingMinor)} left</Text></> : <><Text style={styles.pactAmount}>Progress unavailable</Text><Text style={styles.pactEmpty}>Reconnect to the updated API.</Text></>}</Pressable>; })}</ScrollView> : <Pressable onPress={() => router.push('/pacts' as Href)} style={styles.pactCard}><Text style={styles.pactName}>No active pacts</Text><Text style={styles.pactEmpty}>Your active spending limits will appear here.</Text></Pressable>}</View></View>
        <View style={styles.activitySection}>
          <Pressable onPress={() => router.push('/activity' as Href)} style={styles.activityHeading}><Text style={styles.kicker}>RECENT ACTIVITY —</Text><Text style={styles.activityArrow}>→</Text></Pressable>
          <View onLayout={(event) => setActivityViewportHeight(Math.round(event.nativeEvent.layout.height))} style={styles.activityViewport}>{loading && !data ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : error && !data ? <Pressable onPress={() => void refresh()} style={styles.state}><Text style={styles.stateTitle}>Couldn’t load wallet</Text><Text style={styles.stateCopy}>Tap to try again</Text></Pressable> : activities.length === 0 ? <View style={styles.state}><Text style={styles.stateTitle}>No activity yet</Text><Text style={styles.stateCopy}>Your wallet transactions will appear here.</Text></View> : activities.slice(0, 3).map((item) => <ActivityRow card item={item} key={item.id} previewHeight={activityCardHeight} />)}</View>
        </View>
      </View>
    </View>
  </View>;
}
const styles = StyleSheet.create({
  root: { backgroundColor: colors.white, flex: 1 }, hero: { backgroundColor: colors.blue, borderBottomLeftRadius: radii.sheet, borderBottomRightRadius: radii.sheet, height: 294, overflow: 'hidden', paddingHorizontal: 16, zIndex: 2 }, heroTop: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' }, brand: { color: colors.white, fontFamily: fonts.black, fontSize: 25, letterSpacing: -1.5 }, menuButton: { alignItems: 'center', height: 48, justifyContent: 'center', width: 48 }, balanceWrap: { alignItems: 'center', flex: 1, justifyContent: 'center', paddingBottom: 18 }, available: { color: colors.onBlueMuted, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.7 }, balance: { color: colors.white, fontFamily: fonts.bold, fontSize: 44, letterSpacing: -1.5, marginTop: 6 },
  content: { backgroundColor: colors.white, flex: 1, gap: 22, paddingTop: 22 }, actions: { backgroundColor: colors.white, flexDirection: 'row', gap: 10, paddingHorizontal: 14 }, action: { alignItems: 'center', backgroundColor: colors.softButton, borderRadius: 12, flex: 1, height: 56, justifyContent: 'center' }, actionText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16 }, addAction: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 12, flex: 1, flexDirection: 'row', height: 56, justifyContent: 'center' }, plus: { color: colors.white, fontFamily: fonts.medium, fontSize: 24, marginRight: 4 }, addText: { color: colors.white, fontFamily: fonts.bold, fontSize: 15 }, centerAction: { alignItems: 'center', backgroundColor: colors.navy, borderRadius: 12, height: 56, justifyContent: 'center', width: 56 }, centerIcon: { alignItems: 'center', flexDirection: 'row', gap: 3, justifyContent: 'center' }, centerArrow: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 }, panel: { backgroundColor: colors.surface, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, paddingHorizontal: 14, paddingTop: 20 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 }, dashboardRow: { flexDirection: 'row', gap: 10 }, recentsColumn: { flex: 1 }, pactsColumn: { flex: 1.4 }, recentsGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 10 }, person: { width: '45%' }, avatar: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: 12, height: 64, justifyContent: 'center', width: '100%' }, avatarEmpty: { backgroundColor: colors.softButton }, avatarText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14 }, pactCarousel: { marginTop: 10 }, pactCard: { backgroundColor: colors.white, borderRadius: 14, height: 136, marginRight: 8, marginTop: 10, padding: 14, width: 190 }, pactName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 14 }, pactAmount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16, marginTop: 11 }, pactOf: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10 }, pactTrack: { backgroundColor: 'rgba(90,100,120,0.18)', borderRadius: 4, height: 5, marginTop: 9, overflow: 'hidden' }, pactFill: { backgroundColor: colors.blue, borderRadius: 4, height: 5 }, pactEmpty: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, lineHeight: 15, marginTop: 7 },
  activitySection: { flex: 1, minHeight: 0 }, activityHeading: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginTop: 24, paddingHorizontal: 2, paddingVertical: 3 }, activityArrow: { color: colors.navy, fontFamily: fonts.medium, fontSize: 22 }, activityViewport: { flex: 1, marginTop: 10, minHeight: 0, overflow: 'hidden' },
  loader: { marginTop: 42 }, state: { alignItems: 'center', backgroundColor: colors.white, borderRadius: 16, justifyContent: 'center', minHeight: 100, padding: 18 }, stateTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, stateCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 4, textAlign: 'center' },
});
