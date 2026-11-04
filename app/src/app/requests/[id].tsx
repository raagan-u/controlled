import { router, type Href, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';

import type { PaymentRequest, User, Wallet } from '@/api/types';
import { apiErrorMessage, cancelPaymentRequest, declinePaymentRequest, formatUSD, getCurrentUser, getPaymentRequest, getWallet, payPaymentRequest } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { colors, fonts, radii } from '@/theme';
import { useActionCounts } from '@/notifications/ActionCounts';
import { useActiveRefresh } from '@/hooks/useActiveRefresh';

const initialNow = Date.now();

function expiryLabel(expiresAt: string, now: number) {
  const remaining = new Date(expiresAt).getTime() - now;
  if (remaining <= 0) return 'Expired';
  const totalMinutes = Math.max(1, Math.ceil(remaining / 60_000));
  const days = Math.floor(totalMinutes / 1_440);
  const hours = Math.floor((totalMinutes % 1_440) / 60);
  const minutes = totalMinutes % 60;
  return `Expires in ${[days ? `${days}d` : '', hours ? `${hours}h` : '', `${minutes}m`].filter(Boolean).join(' ')}`;
}

export default function RequestDetailScreen(){
  const {refresh:refreshActionCounts}=useActionCounts();
  const {id}=useLocalSearchParams<{id:string}>();const requestID=typeof id==='string'?id:'';
  const [item,setItem]=useState<PaymentRequest|null>(null);const [user,setUser]=useState<User|null>(null);const [wallet,setWallet]=useState<Wallet|null>(null);const [now,setNow]=useState(initialNow);const [loading,setLoading]=useState(true);const [acting,setActing]=useState(false);const [error,setError]=useState('');
  const load=useCallback(async()=>{if(!requestID)return;try{const [next,current,currentWallet]=await Promise.all([getPaymentRequest(requestID),getCurrentUser(),getWallet().catch(()=>null)]);setItem(next);setUser(current.user);setWallet(currentWallet?.wallet??null);setError('')}catch(cause){setError(apiErrorMessage(cause))}finally{setLoading(false)}},[requestID]);
  useActiveRefresh(load);
  useEffect(()=>{const timer=setInterval(()=>setNow(Date.now()),30_000);return()=>clearInterval(timer)},[]);
  const action=async(kind:'pay'|'decline'|'cancel')=>{if(!item||acting)return;setActing(true);setError('');try{if(kind==='pay'){const result=await payPaymentRequest(item.id);setItem(result.paymentRequest);const nextWallet=await getWallet().catch(()=>null);if(nextWallet)setWallet(nextWallet.wallet)}else if(kind==='decline'){setItem(await declinePaymentRequest(item.id))}else{setItem(await cancelPaymentRequest(item.id))}await refreshActionCounts()}catch(cause){setError(apiErrorMessage(cause))}finally{setActing(false)}};
  if(loading&&!item)return <SheetScreen title="request"><ActivityIndicator color={colors.blue} style={styles.loading}/></SheetScreen>;
  if(!item)return <SheetScreen title="request"><Text style={styles.error}>{error||'Request not found.'}</Text></SheetScreen>;
  const incoming=item.recipient.id===user?.id;const pending=item.status==='PENDING';
	const paid=item.status==='PAID';const declined=item.status==='DECLINED';const waiting=item.status==='PENDING'||item.status==='PROCESSING'||item.status==='AWAITING_APPROVAL';
  return <SheetScreen footer={<View style={styles.actions}>{incoming&&pending?<><SwipeConfirm disabled={acting} key={`${acting}-${error}`} label={acting?'Processing…':'Swipe right to pay'} onConfirm={()=>void action('pay')}/><ActionButton disabled={acting} label="Decline" onPress={()=>void action('decline')} tone="danger"/></>:item.status==='AWAITING_APPROVAL'&&item.accountabilityRequestId?<ActionButton label="View approval request" onPress={()=>router.push(`/accountability/${item.accountabilityRequestId}` as Href)}/>:!incoming&&pending?<ActionButton disabled={acting} label="Cancel request" onPress={()=>void action('cancel')} tone="danger"/>:<ActionButton label="Back to requests" onPress={()=>router.replace('/requests')}/>}</View>} scroll title="request">
    <Text style={styles.kicker}>{incoming?'REQUESTED FROM YOU':'YOUR REQUEST'}</Text><Text style={styles.amount}>${formatUSD(item.amountMinor)}</Text><Text style={styles.person}>{incoming?item.requester.displayName:item.recipient.displayName}</Text><Text style={styles.handle}>{incoming?item.requester.ctrldId:item.recipient.ctrldId}</Text>
	<View style={[styles.status,paid&&styles.statusPaid,declined&&styles.statusDeclined,!paid&&!declined&&!waiting&&styles.statusInactive]}><Text style={[styles.statusText,paid&&styles.statusPaidText,declined&&styles.statusDeclinedText,!paid&&!declined&&!waiting&&styles.statusInactiveText]}>{item.status.replaceAll('_',' ')}</Text></View>{item.note?<Text style={styles.note}>{item.note}</Text>:null}
    {incoming&&waiting&&wallet?<View style={styles.balance}><Text style={styles.balanceLabel}>AVAILABLE BALANCE</Text><Text style={styles.balanceValue}>${formatUSD(wallet.balances.available.amountMinor)}</Text></View>:null}
    <View style={styles.meta}>{waiting?<Text style={styles.metaText}>{expiryLabel(item.expiresAt,now)}</Text>:null}{item.transferId?<Text style={styles.metaText}>Transfer {item.transferId.slice(0,8)}</Text>:null}</View>
    {item.status==='AWAITING_APPROVAL'?<Text style={styles.info}>Your accountability circle must approve this outgoing payment before money moves.</Text>:null}{error?<Text style={styles.error}>{error}</Text>:null}
  </SheetScreen>;
}
const styles=StyleSheet.create({loading:{marginTop:80},kicker:{color:colors.blue,fontFamily:fonts.bold,fontSize:11,letterSpacing:1.5,textAlign:'center'},amount:{color:colors.navy,fontFamily:fonts.bold,fontSize:52,letterSpacing:-2,marginTop:30,textAlign:'center'},person:{color:colors.navy,fontFamily:fonts.bold,fontSize:19,marginTop:14,textAlign:'center'},handle:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:13,marginTop:3,textAlign:'center'},status:{alignSelf:'center',backgroundColor:colors.orangeTint,borderRadius:10,marginTop:20,paddingHorizontal:13,paddingVertical:8},statusText:{color:colors.orangeText,fontFamily:fonts.bold,fontSize:10,letterSpacing:1},statusPaid:{backgroundColor:colors.greenTint},statusPaidText:{color:colors.greenText},statusDeclined:{backgroundColor:colors.redTint},statusDeclinedText:{color:colors.redText},statusInactive:{backgroundColor:colors.surfaceAlt},statusInactiveText:{color:colors.textMuted},note:{backgroundColor:colors.surface,borderRadius:radii.md,color:colors.navy,fontFamily:fonts.medium,fontSize:15,marginTop:24,padding:18,textAlign:'center'},balance:{alignItems:'center',alignSelf:'center',backgroundColor:colors.surface,borderRadius:radii.md,flexDirection:'row',gap:12,marginTop:18,paddingHorizontal:14,paddingVertical:10},balanceLabel:{color:colors.textMuted,fontFamily:fonts.bold,fontSize:9,letterSpacing:1},balanceValue:{color:colors.navy,fontFamily:fonts.bold,fontSize:14},meta:{gap:7,marginTop:14},metaText:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:12,textAlign:'center'},info:{backgroundColor:colors.orangeTint,borderRadius:radii.md,color:colors.orangeText,fontFamily:fonts.medium,fontSize:13,lineHeight:19,marginTop:22,padding:15,textAlign:'center'},error:{color:colors.redText,fontFamily:fonts.medium,fontSize:13,marginTop:18,textAlign:'center'},actions:{gap:9}});
