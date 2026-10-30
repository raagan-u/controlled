import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';

import type { PaymentRequest, User } from '@/api/types';
import { apiErrorMessage, cancelPaymentRequest, declinePaymentRequest, formatUSD, getCurrentUser, getPaymentRequest, payPaymentRequest } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { SheetScreen } from '@/components/SheetScreen';
import { colors, fonts, radii } from '@/theme';

export default function RequestDetailScreen(){
  const {id}=useLocalSearchParams<{id:string}>();const requestID=typeof id==='string'?id:'';
  const [item,setItem]=useState<PaymentRequest|null>(null);const [user,setUser]=useState<User|null>(null);const [loading,setLoading]=useState(true);const [acting,setActing]=useState(false);const [error,setError]=useState('');
  useEffect(()=>{if(!requestID)return;let active=true;void Promise.all([getPaymentRequest(requestID),getCurrentUser()]).then(([next,current])=>{if(active){setItem(next);setUser(current.user)}}).catch((cause)=>{if(active)setError(apiErrorMessage(cause))}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[requestID]);
  const action=async(kind:'pay'|'decline'|'cancel')=>{if(!item||acting)return;setActing(true);setError('');try{if(kind==='pay'){const result=await payPaymentRequest(item.id);setItem(result.paymentRequest)}else if(kind==='decline'){setItem(await declinePaymentRequest(item.id))}else{setItem(await cancelPaymentRequest(item.id))}}catch(cause){setError(apiErrorMessage(cause))}finally{setActing(false)}};
  if(loading&&!item)return <SheetScreen title="request"><ActivityIndicator color={colors.blue} style={styles.loading}/></SheetScreen>;
  if(!item)return <SheetScreen title="request"><Text style={styles.error}>{error||'Request not found.'}</Text></SheetScreen>;
  const incoming=item.recipient.id===user?.id;const pending=item.status==='PENDING';
  return <SheetScreen footer={<View style={styles.actions}>{incoming&&pending?<><ActionButton disabled={acting} label={acting?'Processing…':'Pay request'} onPress={()=>void action('pay')}/><ActionButton disabled={acting} label="Decline" onPress={()=>void action('decline')} tone="danger"/></>:!incoming&&pending?<ActionButton disabled={acting} label="Cancel request" onPress={()=>void action('cancel')} tone="danger"/>:<ActionButton label="Back to requests" onPress={()=>router.replace('/requests')}/>}</View>} scroll title="request">
    <Text style={styles.kicker}>{incoming?'REQUESTED FROM YOU':'YOUR REQUEST'}</Text><Text style={styles.amount}>${formatUSD(item.amountMinor)}</Text><Text style={styles.person}>{incoming?item.requester.displayName:item.recipient.displayName}</Text><Text style={styles.handle}>{incoming?item.requester.ctrldId:item.recipient.ctrldId}</Text>
    <View style={styles.status}><Text style={styles.statusText}>{item.status.replaceAll('_',' ')}</Text></View>{item.note?<Text style={styles.note}>{item.note}</Text>:null}
    <View style={styles.meta}><Text style={styles.metaText}>Created {new Date(item.createdAt).toLocaleString()}</Text><Text style={styles.metaText}>Expires {new Date(item.expiresAt).toLocaleString()}</Text>{item.transferId?<Text style={styles.metaText}>Transfer {item.transferId.slice(0,8)}</Text>:null}</View>
    {item.status==='AWAITING_APPROVAL'?<Text style={styles.info}>Your accountability circle must approve this outgoing payment before money moves.</Text>:null}{error?<Text style={styles.error}>{error}</Text>:null}
  </SheetScreen>;
}
const styles=StyleSheet.create({loading:{marginTop:80},kicker:{color:colors.blue,fontFamily:fonts.bold,fontSize:11,letterSpacing:1.5,textAlign:'center'},amount:{color:colors.navy,fontFamily:fonts.bold,fontSize:52,letterSpacing:-2,marginTop:30,textAlign:'center'},person:{color:colors.navy,fontFamily:fonts.bold,fontSize:19,marginTop:14,textAlign:'center'},handle:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:13,marginTop:3,textAlign:'center'},status:{alignSelf:'center',backgroundColor:colors.blueTint,borderRadius:10,marginTop:20,paddingHorizontal:13,paddingVertical:8},statusText:{color:colors.blue,fontFamily:fonts.bold,fontSize:10,letterSpacing:1},note:{backgroundColor:colors.surface,borderRadius:radii.md,color:colors.navy,fontFamily:fonts.medium,fontSize:15,marginTop:24,padding:18,textAlign:'center'},meta:{gap:7,marginTop:22},metaText:{color:colors.textMuted,fontFamily:fonts.medium,fontSize:12,textAlign:'center'},info:{backgroundColor:colors.orangeTint,borderRadius:radii.md,color:colors.orangeText,fontFamily:fonts.medium,fontSize:13,lineHeight:19,marginTop:22,padding:15,textAlign:'center'},error:{color:colors.redText,fontFamily:fonts.medium,fontSize:13,marginTop:18,textAlign:'center'},actions:{gap:9}});
