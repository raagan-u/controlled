import { useMemo } from 'react';
import { Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import { ActivityRow } from '@/components/ActivityRow';
import { SheetScreen } from '@/components/SheetScreen';
import { transactionToActivity, visibleTransactions } from '@/data/activity';
import { useWalletOverview } from '@/hooks/useWalletOverview';
import { colors, fonts } from '@/theme';

export default function ActivityScreen() {
  const { data, error, loading, refresh } = useWalletOverview();
  const activities = useMemo(() => data ? visibleTransactions(data.transactions).map((item) => transactionToActivity(item, data.wallet)) : [], [data]);
  const sections = [...new Set(activities.map((item) => item.section))];

  return <SheetScreen><ScrollView contentContainerStyle={styles.list} refreshControl={<RefreshControl colors={[colors.blue]} onRefresh={() => void refresh()} refreshing={loading} tintColor={colors.blue} />} showsVerticalScrollIndicator={false}>
    {error && !data ? <Pressable onPress={() => void refresh()} style={styles.state}><Text style={styles.stateTitle}>Couldn’t load activity</Text><Text style={styles.stateCopy}>{error}{'\n'}Tap to try again.</Text></Pressable> : null}
    {!loading && !error && activities.length === 0 ? <View style={styles.state}><Text style={styles.stateTitle}>No activity yet</Text><Text style={styles.stateCopy}>Add money or make a transfer to get started.</Text></View> : null}
    {sections.map((section) => { const items = activities.filter((item) => item.section === section); return <View key={section} style={styles.section}><Text style={styles.kicker}>{section.toUpperCase()} —</Text>{items.map((item, index) => <ActivityRow detailed item={item} key={item.id} last={index === items.length - 1} />)}</View>; })}
  </ScrollView></SheetScreen>;
}

const styles = StyleSheet.create({ list: { flexGrow: 1, paddingBottom: 30 }, section: { marginBottom: 18 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginBottom: 5 }, state: { alignItems: 'center', flex: 1, justifyContent: 'center', minHeight: 300, padding: 28 }, stateTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 18 }, stateCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginTop: 7, textAlign: 'center' } });
