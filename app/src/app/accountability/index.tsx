import { useCallback, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { router, type Href, useFocusEffect } from 'expo-router';
import { FeatureScreen } from '@/components/FeatureScreen';
import { getAccountabilityRequests } from '@/api/pacts';
import { apiErrorMessage, formatUSD, getCurrentUser, getRecipients } from '@/api/wallet';
import type { AccountabilityRequest, User } from '@/api/types';
import { colors, fonts, radii } from '@/theme';

export default function AccountabilityScreen() {
  const [requests, setRequests] = useState<AccountabilityRequest[]>([]); const [users, setUsers] = useState<User[]>([]); const [current, setCurrent] = useState<User | null>(null); const [loading, setLoading] = useState(true); const [error, setError] = useState('');
  const load = useCallback(async () => { try { setError(''); const [items, recipients, me] = await Promise.all([getAccountabilityRequests(), getRecipients(), getCurrentUser()]); setRequests(items); setUsers(recipients.users); setCurrent(me.user); } catch (value) { setError(apiErrorMessage(value)); } finally { setLoading(false); } }, []);
  useFocusEffect(useCallback(() => { void load(); }, [load]));
  const people = useMemo(() => new Map([...(current ? [current] : []), ...users].map((user) => [user.id, user])), [current, users]);
  const needs = requests.filter((item) => item.status === 'ASSIGNED' && item.assignedPartnerId === current?.id);
  const pending = requests.filter((item) => item.status === 'ASSIGNED' && item.ownerId === current?.id);
  const complete = requests.filter((item) => item.status !== 'ASSIGNED');
  return <FeatureScreen title="Accountability">
    {loading ? <ActivityIndicator color={colors.blue} style={styles.loader} /> : null}{error ? <Pressable onPress={() => void load()}><Text style={styles.error}>{error} Tap to retry.</Text></Pressable> : null}
    {!loading && !requests.length ? <View style={styles.empty}><Text style={styles.emptyIcon}>◇</Text><Text style={styles.emptyTitle}>Nothing needs attention</Text><Text style={styles.muted}>Approval requests involving you will appear here.</Text></View> : null}
    <Section items={needs} label="NEEDS YOUR DECISION —" people={people} />
    <Section items={pending} label="YOUR PENDING REQUESTS —" people={people} />
    <Section items={complete} label="COMPLETED —" people={people} />
  </FeatureScreen>;
}
function Section({ items, label, people }: { items: AccountabilityRequest[]; label: string; people: Map<string, User> }) { if (!items.length) return null; return <View><Text style={styles.kicker}>{label}</Text>{items.map((item) => { const owner = people.get(item.ownerId); const destination = [...people.values()].find((user) => user.email === item.destination || user.ctrldId === item.destination); const wording = owner ? `${owner.displayName} wants to ${item.operationType === 'WITHDRAWAL' ? 'withdraw' : 'send'} $${formatUSD(item.amountMinor)}${destination ? ` to ${destination.displayName}` : item.operationType === 'WITHDRAWAL' ? ` to ${item.destination}` : ''}.` : `You want to ${item.operationType === 'WITHDRAWAL' ? 'withdraw' : 'send'} $${formatUSD(item.amountMinor)}${destination ? ` to ${destination.displayName}` : ''}.`; return <Pressable key={item.id} onPress={() => router.push(`/accountability/${item.id}` as Href)} style={styles.request}><View style={[styles.dot, { backgroundColor: statusColor(item.status) }]} /><View style={styles.copy}><Text style={styles.title}>{wording}</Text><Text numberOfLines={1} style={styles.muted}>“{item.reason}”</Text></View><Text style={[styles.badge, { color: statusColor(item.status) }]}>{item.status}</Text></Pressable>; })}</View>; }
function statusColor(status: AccountabilityRequest['status']) { if (status === 'APPROVED') return colors.greenText; if (status === 'DENIED' || status === 'FAILED') return colors.redText; if (status === 'ASSIGNED') return colors.blue; return colors.textMuted; }
const styles = StyleSheet.create({ loader: { marginTop: 70 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginVertical: 18, textAlign: 'center' }, empty: { alignItems: 'center', marginTop: 100, paddingHorizontal: 30 }, emptyIcon: { color: colors.blue, fontFamily: fonts.bold, fontSize: 42 }, emptyTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 21, marginTop: 14 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginBottom: 7, marginTop: 15 }, request: { alignItems: 'center', borderBottomColor: colors.divider, borderBottomWidth: 1, flexDirection: 'row', minHeight: 68 }, dot: { borderRadius: 4, height: 7, marginRight: 10, width: 7 }, copy: { flex: 1, paddingRight: 8 }, title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 13, lineHeight: 18 }, muted: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, lineHeight: 15, marginTop: 2 }, badge: { backgroundColor: colors.surface, borderRadius: radii.tag, fontFamily: fonts.bold, fontSize: 8, overflow: 'hidden', paddingHorizontal: 6, paddingVertical: 4 } });
