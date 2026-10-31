import { useCallback, useEffect, useMemo, useState } from 'react';
import { router } from 'expo-router';
import { Animated, BackHandler, PanResponder, Pressable, RefreshControl, ScrollView, StyleSheet, Text, useWindowDimensions, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ActivityRow } from '@/components/ActivityRow';
import { transactionToActivity, visibleTransactions } from '@/data/activity';
import { useWalletOverview } from '@/hooks/useWalletOverview';
import { colors, fonts, radii } from '@/theme';

export default function ActivityScreen() {
  const { data, error, loading, refresh } = useWalletOverview();
  const insets = useSafeAreaInsets();
  const { height: windowHeight } = useWindowDimensions();
  const sheetTop = Math.max(insets.top + 92, 124);
  const sheetHeight = Math.max(360, windowHeight - sheetTop);
  const [translateY] = useState(() => new Animated.Value(sheetHeight));
  const activities = useMemo(() => data ? visibleTransactions(data.transactions).map((item) => transactionToActivity(item, data.wallet)) : [], [data]);
  const sections = [...new Set(activities.map((item) => item.section))];
  const close = useCallback(() => {
    Animated.timing(translateY, { duration: 220, toValue: sheetHeight, useNativeDriver: true }).start(({ finished }) => {
      if (finished) router.back();
    });
  }, [sheetHeight, translateY]);
  useEffect(() => {
    translateY.setValue(sheetHeight);
    const frame = requestAnimationFrame(() => {
      Animated.timing(translateY, { duration: 240, toValue: 0, useNativeDriver: true }).start();
    });
    return () => cancelAnimationFrame(frame);
  }, [sheetHeight, translateY]);
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => { close(); return true; });
    return () => subscription.remove();
  }, [close]);
  const collapseGesture = useMemo(() => PanResponder.create({
    onMoveShouldSetPanResponder: (_, gesture) => gesture.dy > 8 && Math.abs(gesture.dy) > Math.abs(gesture.dx),
    onPanResponderMove: (_, gesture) => translateY.setValue(Math.max(0, Math.min(sheetHeight, gesture.dy))),
    onPanResponderRelease: (_, gesture) => {
      if (gesture.dy > 64 || gesture.vy > 0.7) close();
      else Animated.spring(translateY, { damping: 20, stiffness: 220, toValue: 0, useNativeDriver: true }).start();
    },
    onPanResponderTerminate: () => Animated.spring(translateY, { damping: 20, stiffness: 220, toValue: 0, useNativeDriver: true }).start(),
  }), [close, sheetHeight, translateY]);

  return <View pointerEvents="box-none" style={styles.overlay}>
    <Animated.View style={[styles.sheet, { height: sheetHeight, paddingBottom: Math.max(insets.bottom, 12), transform: [{ translateY }] }]}>
      <View accessibilityLabel="Swipe down to close activity" accessible {...collapseGesture.panHandlers} style={styles.grabberTouch}><View style={styles.grabber} /></View>
      <View style={styles.heading}><Text style={styles.title}>Activity</Text><Text style={styles.hint}>Swipe down to close</Text></View>
      <ScrollView contentContainerStyle={styles.list} refreshControl={<RefreshControl colors={[colors.blue]} onRefresh={() => void refresh()} refreshing={loading} tintColor={colors.blue} />} showsVerticalScrollIndicator={false}>
        {error && !data ? <Pressable onPress={() => void refresh()} style={styles.state}><Text style={styles.stateTitle}>Couldn’t load activity</Text><Text style={styles.stateCopy}>{error}{'\n'}Tap to try again.</Text></Pressable> : null}
        {!loading && !error && activities.length === 0 ? <View style={styles.state}><Text style={styles.stateTitle}>No activity yet</Text><Text style={styles.stateCopy}>Add money or make a transfer to get started.</Text></View> : null}
        {sections.map((section) => { const items = activities.filter((item) => item.section === section); return <View key={section} style={styles.section}><Text style={styles.kicker}>{section.toUpperCase()} —</Text>{items.map((item, index) => <ActivityRow detailed item={item} key={item.id} last={index === items.length - 1} />)}</View>; })}
      </ScrollView>
    </Animated.View>
  </View>;
}

const styles = StyleSheet.create({ overlay: { flex: 1, justifyContent: 'flex-end' }, sheet: { backgroundColor: colors.white, borderTopLeftRadius: radii.sheet, borderTopRightRadius: radii.sheet, elevation: 24, overflow: 'hidden', paddingHorizontal: 16, shadowColor: colors.navy, shadowOffset: { height: -8, width: 0 }, shadowOpacity: 0.18, shadowRadius: 18 }, grabberTouch: { alignItems: 'center', height: 40, justifyContent: 'center' }, grabber: { backgroundColor: colors.grabber, borderRadius: 3, height: 4, width: 36 }, heading: { alignItems: 'baseline', flexDirection: 'row', justifyContent: 'space-between', paddingBottom: 14 }, title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 26, letterSpacing: -0.6 }, hint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11 }, list: { flexGrow: 1, paddingBottom: 30 }, section: { marginBottom: 18 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginBottom: 5 }, state: { alignItems: 'center', flex: 1, justifyContent: 'center', minHeight: 300, padding: 28 }, stateTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 18 }, stateCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginTop: 7, textAlign: 'center' } });
