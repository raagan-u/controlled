import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { router, type Href, useFocusEffect } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { PactCard } from '@/components/PactCard';
import { getPacts } from '@/api/pacts';
import type { Pact } from '@/api/types';
import { colors, fonts, radii } from '@/theme';

type Filter = 'ALL' | 'KEPT' | 'BROKEN' | 'CANCELLED';
export default function PactHistoryScreen() {
  const [pacts, setPacts] = useState<Pact[]>([]); const [filter, setFilter] = useState<Filter>('ALL'); const [loading, setLoading] = useState(true);
  useFocusEffect(useCallback(() => { void getPacts().then((items) => setPacts(items.filter((pact) => pact.status === 'RESOLVED' || pact.status === 'CANCELLED'))).finally(() => setLoading(false)); }, []));
  const filtered = pacts.filter((pact) => filter === 'ALL' || (filter === 'KEPT' ? pact.outcome.startsWith('KEPT') : pact.outcome === filter));
  return <FeatureScreen menu onMenu={() => router.replace('/home?openMenu=1' as Href)} title="Pact history">
    <View style={styles.filters}>{(['ALL', 'KEPT', 'BROKEN', 'CANCELLED'] as Filter[]).map((item) => <Pressable key={item} onPress={() => setFilter(item)} style={[styles.filter, filter === item && styles.filterActive]}><Text style={[styles.filterText, filter === item && styles.filterTextActive]}>{item[0] + item.slice(1).toLowerCase()}</Text></Pressable>)}</View>
    {loading ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : filtered.length ? filtered.map((pact) => <PactCard compact key={pact.id} onPress={() => router.push(`/pacts/${pact.id}` as Href)} pact={pact} />) : <View style={styles.empty}><Text style={styles.emptyTitle}>No matching pacts</Text><Text style={styles.emptyText}>Completed and cancelled pacts will stay here.</Text></View>}
  </FeatureScreen>;
}
const styles = StyleSheet.create({ filters: { flexDirection: 'row', gap: 7, marginBottom: 20 }, filter: { borderColor: colors.divider, borderRadius: radii.pill, borderWidth: 1, paddingHorizontal: 12, paddingVertical: 8 }, filterActive: { backgroundColor: colors.navy, borderColor: colors.navy }, filterText: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 11 }, filterTextActive: { color: colors.white }, loader: { marginTop: 70 }, empty: { alignItems: 'center', marginTop: 90 }, emptyTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 20 }, emptyText: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 7 } });
