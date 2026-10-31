import { router, type Href } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { PaymentRequest, User } from '@/api/types';
import { apiErrorMessage, formatUSD, getCurrentUser, getPaymentRequests } from '@/api/wallet';
import { SheetScreen } from '@/components/SheetScreen';
import { colors, fonts, radii } from '@/theme';

const statusLabels: Record<PaymentRequest['status'], string> = { PENDING: 'Pending', PROCESSING: 'Processing', AWAITING_APPROVAL: 'Awaiting approval', PAID: 'Paid', DECLINED: 'Declined', CANCELLED: 'Cancelled', EXPIRED: 'Expired' };
function statusStyle(status: PaymentRequest['status']) {
	if (status === 'PAID') return styles.paid;
	if (status === 'DECLINED') return styles.declined;
	if (status === 'PENDING' || status === 'PROCESSING' || status === 'AWAITING_APPROVAL') return styles.pending;
	return styles.inactive;
}

export default function RequestsScreen() {
  const [items, setItems] = useState<PaymentRequest[]>([]);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
	await Promise.resolve();
    setError('');
    try {
      const [requests, current] = await Promise.all([getPaymentRequests(), getCurrentUser()]);
      setItems(requests); setUser(current.user);
    } catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => {
	let active = true;
	void Promise.all([getPaymentRequests(), getCurrentUser()])
	  .then(([requests, current]) => { if (active) { setItems(requests); setUser(current.user); } })
	  .catch((cause) => { if (active) setError(apiErrorMessage(cause)); })
	  .finally(() => { if (active) setLoading(false); });
	return () => { active = false; };
  }, []);

  return <SheetScreen title="requests">
    <View style={styles.heading}><View><Text style={styles.title}>Money requests</Text><Text style={styles.subtitle}>Incoming and sent requests</Text></View><Pressable onPress={() => router.push('/request-money' as Href)} style={styles.newButton}><Text style={styles.newText}>＋ New</Text></Pressable></View>
    {loading && items.length === 0 ? <ActivityIndicator color={colors.blue} style={styles.loading} /> : null}
    {error ? <Pressable onPress={() => void load()}><Text style={styles.error}>{error} Tap to retry.</Text></Pressable> : null}
    {!loading && !error && items.length === 0 ? <View style={styles.empty}><Text style={styles.emptyTitle}>No requests yet</Text><Text style={styles.emptyCopy}>Requests you send or receive will appear here.</Text></View> : null}
    <ScrollView refreshControl={<RefreshControl refreshing={loading} onRefresh={() => void load()} />} showsVerticalScrollIndicator={false} style={styles.list}>
      {items.map((item) => {
        const incoming = item.recipient.id === user?.id;
        const other = incoming ? item.requester : item.recipient;
        return <Pressable key={item.id} onPress={() => router.push(`/requests/${item.id}` as Href)} style={styles.card}>
          <View style={styles.cardTop}><Text style={styles.name}>{other.displayName}</Text><Text style={styles.amount}>${formatUSD(item.amountMinor)}</Text></View>
		  <View style={styles.statusRow}><Text style={styles.direction}>{incoming ? 'Requested from you' : 'You requested'} · </Text><Text style={[styles.statusLabel, statusStyle(item.status)]}>{statusLabels[item.status]}</Text></View>
          {item.note ? <Text numberOfLines={1} style={styles.note}>{item.note}</Text> : null}
        </Pressable>;
      })}
    </ScrollView>
  </SheetScreen>;
}

const styles=StyleSheet.create({heading:{alignItems:'center',flexDirection:'row',justifyContent:'space-between',marginBottom:16},title:{color:colors.navy,fontFamily:fonts.bold,fontSize:24},subtitle:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:12,marginTop:3},newButton:{backgroundColor:colors.blue,borderRadius:12,paddingHorizontal:14,paddingVertical:10},newText:{color:colors.white,fontFamily:fonts.bold,fontSize:13},list:{flex:1},card:{backgroundColor:colors.surface,borderRadius:radii.md,marginBottom:10,padding:16},cardTop:{alignItems:'center',flexDirection:'row',justifyContent:'space-between'},name:{color:colors.navy,fontFamily:fonts.bold,fontSize:16},amount:{color:colors.navy,fontFamily:fonts.bold,fontSize:18},statusRow:{alignItems:'center',flexDirection:'row',marginTop:7},direction:{color:colors.textMuted,fontFamily:fonts.bold,fontSize:11,textTransform:'uppercase'},statusLabel:{fontFamily:fonts.bold,fontSize:11,textTransform:'uppercase'},paid:{color:colors.greenText},declined:{color:colors.redText},pending:{color:colors.orangeText},inactive:{color:colors.textMuted},note:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:13,marginTop:6},loading:{marginTop:50},error:{color:colors.redText,fontFamily:fonts.medium,fontSize:13,padding:20,textAlign:'center'},empty:{alignItems:'center',backgroundColor:colors.surface,borderRadius:radii.md,marginTop:20,padding:30},emptyTitle:{color:colors.navy,fontFamily:fonts.bold,fontSize:16},emptyCopy:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:13,marginTop:6,textAlign:'center'}});
