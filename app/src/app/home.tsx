import { router, type Href, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, BackHandler, PanResponder, ScrollView, type ScrollViewInstance, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import Animated, { FadeIn, FadeInDown, interpolate, useAnimatedStyle, useDerivedValue, useSharedValue, withSpring } from 'react-native-reanimated';
import { PressableScale, rise, useCountUp } from '@/motion';
import { ActivityRow } from '@/components/ActivityRow';
import { HeroShapes } from '@/components/HeroShapes';
import { AuthenticatedMenu } from '@/components/AuthenticatedMenu';
import { formatUSD } from '@/api/wallet';
import { transactionToActivity, visibleTransactions } from '@/data/activity';
import { useWalletOverview } from '@/hooks/useWalletOverview';
import { colors, fonts, radii } from '@/theme';
import { getPacts } from '@/api/pacts';
import type { Pact } from '@/api/types';

const HERO_HEIGHT = 294;

export default function HomeScreen() {
  const { openActivity, openMenu } = useLocalSearchParams<{ openActivity?: string; openMenu?: string }>();
  const { data, error, loading, refresh } = useWalletOverview();
  const balanceMinor = useCountUp(data?.wallet.balances.available.amountMinor);
  const balance = data ? formatUSD(Math.round(balanceMinor)) : '—';
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
  const [pactWidth, setPactWidth] = useState(0);
  useFocusEffect(useCallback(() => { void getPacts().then((items) => setPacts(items.filter((pact) => pact.status === 'ACTIVE' || pact.status === 'SCHEDULED'))).catch(() => setPacts([])); }, []));
  const insets = useSafeAreaInsets();
  // The panel is an absolute layer that rests over the "slot" the old in-flow panel occupied, and
  // slides up over the hero as one surface. `y` is its translateY: collapsedOffset (resting) .. 0 (open).
  const sheetTop = Math.max(insets.top + 92, 124);
  const [slot, setSlot] = useState<{ top: number } | null>(null);
  const [sectionY, setSectionY] = useState<number | null>(null);
  const [fullViewport, setFullViewport] = useState(0);
  const [expanded, setExpanded] = useState(false);
  const [atTop, setAtTop] = useState(true);
  const collapsedOffset = slot ? Math.max(1, slot.top - sheetTop) : 1;
  const sectionLift = Math.max(0, (sectionY ?? 0) - 18);
  // The full list area is taller than the resting viewport by the panel growth plus the section lift.
  const restingViewport = Math.max(1, fullViewport - collapsedOffset - sectionLift);
  const ready = slot !== null && sectionY !== null && fullViewport > 0;
  const activityCardHeight = ready ? Math.max(68, Math.min(94, (restingViewport - 16) / 2.25)) : undefined;
  const y = useSharedValue(0);
  const listRef = useRef<ScrollViewInstance>(null);
  const geometry = useSharedValue({ full: 1, lift: 0, offset: 1, resting: 1 });
  useEffect(() => { geometry.set({ full: Math.max(1, fullViewport), lift: sectionLift, offset: collapsedOffset, resting: restingViewport }); }, [collapsedOffset, fullViewport, geometry, restingViewport, sectionLift]);
  const progress = useDerivedValue(() => interpolate(y.value, [0, geometry.value.offset], [1, 0], 'clamp'));
  const panelMotion = useAnimatedStyle(() => ({ transform: [{ translateY: y.value }] }));
  const dashboardFade = useAnimatedStyle(() => ({ opacity: Math.max(0, 1 - progress.value * 2.4) }));
  const sectionMotion = useAnimatedStyle(() => ({ transform: [{ translateY: -progress.value * geometry.value.lift }] }));
  const clipMotion = useAnimatedStyle(() => ({ height: geometry.value.resting + (geometry.value.full - geometry.value.resting) * progress.value }));
  const arrowFade = useAnimatedStyle(() => ({ opacity: 1 - progress.value }));
  const grabberFade = useAnimatedStyle(() => ({ opacity: progress.value }));
  const heroFade = useAnimatedStyle(() => ({ opacity: Math.max(0, 1 - progress.value * 1.6) }));
  // Collapsing returns the list to its first card so it never rests scrolled under the clip.
  useEffect(() => { if (!expanded) listRef.current?.scrollTo({ animated: true, y: 0 }); }, [expanded]);
  // Overshoot is clamped so the panel never lifts off the bottom edge while settling.
  const settle = useCallback((open: boolean) => {
    setExpanded(open);
    y.set(withSpring(open ? 0 : collapsedOffset, { damping: 28, mass: 0.9, overshootClamping: true, stiffness: 190 }));
  }, [collapsedOffset, y]);
  // The menu's Activity item lands here with a fresh `openActivity` stamp; expand once per stamp.
  const [handledStamp, setHandledStamp] = useState<string | undefined>(undefined);
  if (ready && openActivity !== handledStamp) {
    setHandledStamp(openActivity);
    if (openActivity) setExpanded(true);
  }
  useEffect(() => { if (expanded) y.set(withSpring(0, { damping: 28, mass: 0.9, overshootClamping: true, stiffness: 190 })); }, [expanded, y]);
  // Only while Home is focused, so back on a screen pushed above it isn't swallowed here.
  useFocusEffect(useCallback(() => {
    if (!expanded) return;
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => { settle(false); return true; });
    return () => subscription.remove();
  }, [expanded, settle]));
  // Collapsed: drag up anywhere on the panel. Open: drag down from the header area at any scroll position,
  // or from anywhere once the list is at its top.
  const panelPan = useMemo(() => PanResponder.create({
    onMoveShouldSetPanResponder: (_, gesture) => !expanded && gesture.dy < -12 && Math.abs(gesture.dy) > Math.abs(gesture.dx),
    onMoveShouldSetPanResponderCapture: (_, gesture) => expanded && (atTop || gesture.y0 - sheetTop < 96) && gesture.dy > 8 && gesture.dy > Math.abs(gesture.dx) * 1.5,
    onPanResponderMove: (_, gesture) => y.set(Math.max(0, Math.min(collapsedOffset, expanded ? gesture.dy - 8 : collapsedOffset + gesture.dy + 12))),
    onPanResponderRelease: (_, gesture) => settle(expanded ? !(gesture.dy > 90 || gesture.vy > 0.7) : gesture.dy < -60 || gesture.vy < -0.6),
    onPanResponderTerminate: () => settle(expanded),
    onPanResponderTerminationRequest: () => false,
  }), [atTop, collapsedOffset, expanded, settle, sheetTop, y]);
  return <View style={styles.root}>
    <View style={[styles.hero, { paddingTop: Math.max(insets.top + 10, 28) }]}>
      <HeroShapes />
      <View style={styles.heroTop}><View style={styles.menuButton} /><Text style={styles.brand}>ctrld</Text><View style={styles.menuButton}><AuthenticatedMenu initiallyOpen={openMenu === '1'} /></View></View>
      <Animated.View entering={FadeInDown.delay(100).duration(500)} style={styles.balanceWrap}><Animated.View style={[styles.balanceInner, heroFade]}><Text style={styles.available}>AVAILABLE</Text><Text adjustsFontSizeToFit numberOfLines={1} style={styles.balance}>${balance}</Text></Animated.View></Animated.View>
    </View>
    <View style={styles.content}>
      <Animated.View entering={rise(0, 150)} style={styles.actions}><PressableScale onPress={() => router.push('/fund' as Href)} scaleTo={0.95} style={styles.addAction}><Text style={styles.plus}>＋</Text><Text style={styles.addText}>Add money</Text></PressableScale><PressableScale accessibilityLabel="Send or request money" onPress={() => router.push('/send' as Href)} scaleTo={0.9} style={styles.centerAction}><View style={styles.centerIcon}><Text style={styles.centerArrow}>↑</Text><Text style={styles.centerArrow}>↓</Text></View></PressableScale><PressableScale onPress={() => router.push('/withdraw' as Href)} scaleTo={0.95} style={styles.action}><Text style={styles.actionText}>Withdraw</Text></PressableScale></Animated.View>
      <View onLayout={(event) => { if (!slot) y.set(HERO_HEIGHT + event.nativeEvent.layout.y - sheetTop); setSlot({ top: HERO_HEIGHT + event.nativeEvent.layout.y }); }} style={styles.slot} />
    </View>
    {slot ? <Animated.View entering={FadeInDown.delay(220).duration(520).springify().damping(20).stiffness(130)} pointerEvents="box-none" style={[styles.panelFrame, { top: sheetTop }]}>
      <Animated.View {...panelPan.panHandlers} style={[styles.panel, { paddingBottom: Math.max(insets.bottom + 6, 14) }, panelMotion]}>
        <Animated.View pointerEvents="none" style={[styles.panelGrabber, grabberFade]} />
        <Animated.View pointerEvents={expanded ? 'none' : 'auto'} style={[styles.dashboardRow, dashboardFade]}><View style={styles.recentsColumn}><Text style={styles.kicker}>RECENTS —</Text><View style={styles.recentsGrid}>{Array.from({ length: 4 }, (_, index) => recentUsers[index]).map((user, index) => user ? <PressableScale accessibilityLabel={`Send money to ${user.displayName}`} entering={FadeIn.delay(380 + index * 70)} key={user.id} onPress={() => router.push(`/send?recipient=${encodeURIComponent(user.ctrldId)}` as Href)} scaleTo={0.9} style={styles.person}><View style={styles.avatar}><Text style={styles.avatarText}>{user.displayName.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()}</Text></View></PressableScale> : <View key={`empty-${index}`} style={styles.person}><View style={[styles.avatar, styles.avatarEmpty]} /></View>)}</View></View><View onLayout={(event) => setPactWidth(event.nativeEvent.layout.width)} style={styles.pactsColumn}><Text style={styles.kicker}>ACTIVE PACTS —</Text>{pacts.length ? <ScrollView horizontal pagingEnabled showsHorizontalScrollIndicator={false} style={styles.pactCarousel}>{pacts.map((pact) => { const available = pact.progressAvailable !== false; const ratio = available ? pact.progress.usedMinor / pact.terms.spendingLimit.amountMinor : 0; const backgroundColor = !available ? colors.surface : pact.progress.overageMinor ? colors.redTint : ratio >= 0.8 ? colors.orangeTint : ratio < 0.5 ? colors.greenTint : colors.blueTint; return <PressableScale key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} scaleTo={0.97} style={[styles.pactCard, pactWidth ? { width: pactWidth } : null, { backgroundColor }]}><Text numberOfLines={2} style={styles.pactName}>{pact.terms.goal}</Text>{available ? <><Text style={styles.pactAmount}>${formatUSD(pact.progress.usedMinor)} <Text style={styles.pactOf}>of ${formatUSD(pact.terms.spendingLimit.amountMinor)}</Text></Text><View style={styles.pactTrack}><View style={[styles.pactFill, { width: `${Math.max(4, Math.min(100, ratio * 100))}%` }]} /></View><Text style={styles.pactEmpty}>${formatUSD(pact.progress.remainingMinor)} left</Text></> : <><Text style={styles.pactAmount}>Progress unavailable</Text><Text style={styles.pactEmpty}>Reconnect to the updated API.</Text></>}</PressableScale>; })}</ScrollView> : <PressableScale onPress={() => router.push('/pacts' as Href)} scaleTo={0.97} style={styles.pactCard}><Text style={styles.pactName}>No active pacts</Text><Text style={styles.pactEmpty}>Your active spending limits will appear here.</Text></PressableScale>}</View></Animated.View>
        <Animated.View onLayout={(event) => setSectionY(Math.round(event.nativeEvent.layout.y))} style={[styles.activitySection, { marginBottom: -sectionLift }, sectionMotion]}>
          <PressableScale onPress={() => settle(!expanded)} scaleTo={0.97} style={styles.activityHeading}><Text style={styles.kicker}>RECENT ACTIVITY —</Text><Animated.Text style={[styles.activityArrow, arrowFade]}>→</Animated.Text></PressableScale>
          <View onLayout={(event) => setFullViewport(Math.round(event.nativeEvent.layout.height))} style={styles.activityViewport}>
            {ready ? <Animated.View style={[styles.activityClip, clipMotion]}>
              <ScrollView contentContainerStyle={styles.activityList} ref={listRef} onScroll={(event) => setAtTop(event.nativeEvent.contentOffset.y <= 0)} overScrollMode="never" scrollEnabled={expanded} scrollEventThrottle={16} showsVerticalScrollIndicator={false} style={{ height: fullViewport }}>
                {loading && !data ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : error && !data ? <PressableScale onPress={() => void refresh()} scaleTo={0.97} style={styles.state}><Text style={styles.stateTitle}>Couldn’t load wallet</Text><Text style={styles.stateCopy}>Tap to try again</Text></PressableScale> : activities.length === 0 ? <View style={styles.state}><Text style={styles.stateTitle}>No activity yet</Text><Text style={styles.stateCopy}>Your wallet transactions will appear here.</Text></View> : activities.slice(0, 60).map((item, index) => <ActivityRow card index={index} item={item} key={item.id} previewHeight={activityCardHeight} />)}
              </ScrollView>
            </Animated.View> : null}
          </View>
        </Animated.View>
      </Animated.View>
    </Animated.View> : null}
  </View>;
}
const styles = StyleSheet.create({
  root: { backgroundColor: colors.white, flex: 1 }, hero: { backgroundColor: colors.blue, borderBottomLeftRadius: radii.sheet, borderBottomRightRadius: radii.sheet, height: HERO_HEIGHT, overflow: 'hidden', paddingHorizontal: 16, zIndex: 2 }, heroTop: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between' }, brand: { color: colors.white, fontFamily: fonts.black, fontSize: 32, letterSpacing: -2 }, menuButton: { alignItems: 'center', height: 48, justifyContent: 'center', width: 48 }, balanceWrap: { alignItems: 'center', flex: 1, justifyContent: 'center', paddingTop: 10 }, balanceInner: { alignItems: 'center' }, available: { color: colors.onBlueMuted, fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.7 }, balance: { color: colors.white, fontFamily: fonts.bold, fontSize: 44, letterSpacing: -1.5, marginTop: 6 },
  content: { backgroundColor: colors.white, flex: 1, gap: 22, paddingTop: 22 }, actions: { backgroundColor: colors.white, flexDirection: 'row', gap: 10, paddingHorizontal: 14 }, action: { alignItems: 'center', backgroundColor: colors.softButton, borderRadius: 12, flex: 1, height: 56, justifyContent: 'center' }, actionText: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16 }, addAction: { alignItems: 'center', backgroundColor: colors.blue, borderRadius: 12, flex: 1, flexDirection: 'row', height: 56, justifyContent: 'center' }, plus: { color: colors.white, fontFamily: fonts.medium, fontSize: 24, marginRight: 4 }, addText: { color: colors.white, fontFamily: fonts.bold, fontSize: 15 }, centerAction: { alignItems: 'center', backgroundColor: colors.navy, borderRadius: 12, height: 56, justifyContent: 'center', width: 56 }, centerIcon: { alignItems: 'center', flexDirection: 'row', gap: 3, justifyContent: 'center' }, centerArrow: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 }, slot: { flex: 1 }, panelFrame: { bottom: 0, left: 0, position: 'absolute', right: 0, zIndex: 5 }, panel: { backgroundColor: colors.surface, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, flex: 1, paddingHorizontal: 14, paddingTop: 20 }, panelGrabber: { alignSelf: 'center', backgroundColor: colors.grabber, borderRadius: 3, height: 4, position: 'absolute', top: 9, width: 36 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 }, dashboardRow: { flexDirection: 'row', gap: 10 }, recentsColumn: { flex: 1 }, pactsColumn: { flex: 1.4, gap: 10 }, recentsGrid: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 10 }, person: { width: '45%' }, avatar: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: 12, height: 64, justifyContent: 'center', width: '100%' }, avatarEmpty: { backgroundColor: colors.softButton }, avatarText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 14 }, pactCarousel: {}, pactCard: { backgroundColor: colors.white, borderRadius: 14, height: 136, padding: 16, width: '100%' }, pactName: { color: colors.navy, fontFamily: fonts.bold, fontSize: 17 }, pactAmount: { color: colors.navy, fontFamily: fonts.bold, fontSize: 21, marginTop: 10 }, pactOf: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12 }, pactTrack: { backgroundColor: 'rgba(90,100,120,0.18)', borderRadius: 4, height: 6, marginTop: 10, overflow: 'hidden' }, pactFill: { backgroundColor: colors.blue, borderRadius: 4, height: 6 }, pactEmpty: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 17, marginTop: 8 },
  activitySection: { flex: 1, minHeight: 0 }, activityHeading: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginTop: 24, paddingHorizontal: 2, paddingVertical: 3 }, activityArrow: { color: colors.navy, fontFamily: fonts.medium, fontSize: 22 }, activityViewport: { flex: 1, marginTop: 10, minHeight: 0 }, activityClip: { overflow: 'hidden' }, activityList: { paddingBottom: 4 },
  loader: { marginTop: 42 }, state: { alignItems: 'center', backgroundColor: colors.white, borderRadius: 16, justifyContent: 'center', minHeight: 100, padding: 18 }, stateTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 15 }, stateCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 4, textAlign: 'center' },
});
