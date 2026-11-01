import { router, type Href, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Image, Linking, StyleSheet, Switch, Text, TextInput, View } from 'react-native';
import type { PriceCheck, User } from '@/api/types';
import { apiErrorMessage, checkPurchasePrice, createTransfer, formatUSD, getWallet, toMinorUnits } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { MoneyAmount } from '@/components/MoneyAmount';
import { RecipientPicker } from '@/components/RecipientPicker';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { TransferTabs } from '@/components/TransferTabs';
import { colors, fonts, radii } from '@/theme';
import { APIError } from '@/api/client';

type Step = 'recipient' | 'amount' | 'purchaseReview' | 'reason' | 'success' | 'review';

export default function SendScreen() {
  const params = useLocalSearchParams<{ recipient?: string }>();
	const preset = typeof params.recipient === 'string' ? params.recipient.trim() : '';
  const [step, setStep] = useState<Step>('recipient');
  const [recipient, setRecipient] = useState<User | null>(null);
  const [amount, setAmount] = useState('');
  const [displayAmount, setDisplayAmount] = useState('');
  const [calculation, setCalculation] = useState<string | null>(null);
  const [remark, setRemark] = useState('');
	const [isPurchase, setIsPurchase] = useState(false);
	const [priceCheck, setPriceCheck] = useState<PriceCheck | null>(null);
	const [checkingPrice, setCheckingPrice] = useState(false);
	const [priceStatus, setPriceStatus] = useState('');
  const [available, setAvailable] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [reviewId, setReviewId] = useState('');
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const valid = amountMinor > 0 && !over && recipient !== null && (!isPurchase || remark.trim().length >= 3);

  useEffect(() => { void getWallet().then(({ wallet }) => setAvailable(wallet.balances.available.amountMinor)).catch((cause) => setError(apiErrorMessage(cause))); }, []);

	useEffect(() => {
		const query = remark.trim();
		if (!isPurchase || query.length < 3 || amountMinor <= 0) return;
		let cancelled = false;
		let requestTimer: ReturnType<typeof setTimeout> | undefined;
		const stateTimer = setTimeout(() => {
			if (cancelled) return;
			setPriceCheck(null); setCheckingPrice(true); setPriceStatus('Checking current prices…');
			requestTimer = setTimeout(() => {
				void checkPurchasePrice(query, amountMinor)
					.then((result) => {
						if (cancelled) return;
						setPriceCheck(result);
						setPriceStatus(result.matched ? `Matched ${result.productTitle}` : result.message);
					})
					.catch(() => { if (!cancelled) setPriceStatus('Price check unavailable — your payment still works.'); })
					.finally(() => { if (!cancelled) setCheckingPrice(false); });
			}, 650);
		}, 0);
		return () => { cancelled = true; clearTimeout(stateTimer); if (requestTimer) clearTimeout(requestTimer); };
	}, [amountMinor, isPurchase, remark]);

  async function send(skipPurchaseReview = false) {
    if (!valid || submitting) return;
		if (isPurchase && priceCheck?.matched && !skipPurchaseReview) { setStep('purchaseReview'); return; }
    setSubmitting(true); setError('');
    try {
		  if (!recipient) return;
      const result = await createTransfer(recipient.ctrldId, amountMinor, remark.trim() || undefined);
      if ('accountabilityRequest' in result) { setReviewId(result.accountabilityRequest.id); setStep('review'); } else setStep('success');
    } catch (cause) { if (cause instanceof APIError && cause.code === 'policy_denied' && !remark.trim()) setStep('reason'); else setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  if (step === 'success') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Done</Text>}><ResultState amount={amount} detail={`Sent to ${recipient?.displayName}${remark ? ` · ${remark}` : ''}`} title="Money sent" /></SheetScreen>;
  if (step === 'review') return <SheetScreen footer={<><Text onPress={() => reviewId && router.replace(`/accountability/${reviewId}` as Href)} style={styles.done}>View request</Text><Text onPress={() => router.replace('/home')} style={styles.backHome}>Back home</Text></>}><ResultState amount={amount} detail="Nothing is reserved while you wait. Your assigned partner has six hours before it moves to the next person." title="Your transaction needs approval" tone="pending" /></SheetScreen>;
	if (step === 'purchaseReview' && priceCheck?.matched) return <SheetScreen footer={<View style={styles.reviewActions}>{priceCheck.offerUrl ? <ActionButton label={`View $${formatUSD(priceCheck.bestPriceMinor ?? 0)} option`} onPress={() => void Linking.openURL(priceCheck.offerUrl!)} tone="soft" /> : null}<SwipeConfirm disabled={submitting} label={submitting ? 'Sending…' : `Swipe to continue with $${formatUSD(amountMinor)}`} onConfirm={() => void send(true)} /><Text onPress={() => setStep('amount')} style={styles.backHome}>Edit purchase</Text></View>} scroll title="price check"><View style={styles.purchaseReview}><Text style={styles.reviewKicker}>MATCHED PRODUCT —</Text><View style={styles.productCard}>{priceCheck.imageUrl ? <Image resizeMode="contain" source={{ uri: priceCheck.imageUrl }} style={styles.productImage} /> : <View style={styles.productFallback}><Text style={styles.productFallbackText}>{priceCheck.productTitle?.slice(0, 1) ?? 'P'}</Text></View>}<View style={styles.productCopy}><Text style={styles.productTitle}>{priceCheck.productTitle}</Text><Text style={styles.productMeta}>{[priceCheck.brand, priceCheck.merchant].filter(Boolean).join(' · ')}</Text></View></View><View style={[styles.insightCard, priceCheck.verdict === 'better_price' && styles.insightSaving]}><Text style={styles.insightTitle}>{priceCheck.verdict === 'better_price' ? `Potentially save $${formatUSD(priceCheck.savingsMinor ?? 0)}` : priceCheck.verdict === 'below_market' ? 'Your price looks strong' : 'Your price looks competitive'}</Text><Text style={styles.insightCopy}>You’re about to pay ${formatUSD(amountMinor)}. The best matching new, in-stock offer we found is ${formatUSD(priceCheck.bestPriceMinor ?? 0)} at {priceCheck.merchant}.</Text></View><Text style={styles.reviewNote}>Confirm the product match before acting on this comparison. Availability and prices can change.</Text></View></SheetScreen>;
  if (step === 'reason') return <SheetScreen><View style={styles.reasonScreen}><View><Text style={styles.warning}>△</Text><Text style={styles.reasonTitle}>This goes over your pact</Text><Text style={styles.reasonCopy}>Explain why you need this transaction. Your accountability circle will see this message.</Text><TextInput maxLength={300} multiline onChangeText={setRemark} placeholder="What happened?" placeholderTextColor={colors.textMuted} style={styles.reasonInput} value={remark} /></View><SwipeConfirm disabled={!remark.trim() || submitting} label={submitting ? 'Creating request…' : 'Swipe right to ask'} onConfirm={() => void send()} /></View></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><TransferTabs active="Send" /><RecipientPicker initialQuery={preset} onSelect={(user) => { setRecipient(user); setError(''); setStep('amount'); }} /></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><View><TransferTabs active="Send" /><Text style={styles.payee}>paying to <Text onPress={() => setStep('recipient')} style={styles.payeeStrong}>{recipient?.displayName}⌄</Text></Text></View><View style={styles.amountBlock}><MoneyAmount calculation={calculation} caption={`Available $${available === null ? '—' : formatUSD(available)}`} onCalculationPress={() => { if (calculation) { setAmount(calculation); setDisplayAmount(calculation); setCalculation(null); setPriceCheck(null); setPriceStatus(''); } }} tone={over ? 'danger' : 'default'} value={displayAmount} /><TextInput maxLength={80} onChangeText={(value) => { setRemark(value); setPriceCheck(null); setPriceStatus(''); }} placeholder={isPurchase ? 'Product name and model' : 'Add a remark'} placeholderTextColor={colors.textMuted} style={styles.remark} value={remark} /><View style={styles.purchaseToggle}><View style={styles.purchaseToggleCopy}><Text style={styles.purchaseToggleTitle}>This is a purchase</Text><Text style={styles.purchaseToggleHint}>Check the price before sending</Text></View><Switch onValueChange={(value) => { setIsPurchase(value); setPriceCheck(null); setPriceStatus(''); }} thumbColor={colors.white} trackColor={{ false: colors.grabber, true: colors.blue }} value={isPurchase} /></View>{isPurchase ? <View style={[styles.priceStatus, priceCheck?.matched && styles.priceStatusMatched]}><Text numberOfLines={2} style={[styles.priceStatusText, priceCheck?.matched && styles.priceStatusMatchedText]}>{remark.trim().length < 3 ? 'Add an exact product name and model.' : checkingPrice ? 'Checking current prices…' : priceStatus || 'Ready to check current prices.'}</Text>{priceCheck?.matched && priceCheck.bestPriceMinor ? <Text style={styles.priceStatusPrice}>from ${formatUSD(priceCheck.bestPriceMinor)}</Text> : null}</View> : null}{over ? <Text style={styles.error}>More than your available balance.</Text> : error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting || (isPurchase && checkingPrice)} key={`${submitting}-${error}`} label={submitting ? 'Sending…' : isPurchase && checkingPrice ? 'Checking price…' : undefined} onConfirm={() => void send()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={(value) => { setAmount(value); setPriceCheck(null); setPriceStatus(''); }} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, payee: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, payeeStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { flex: 1, justifyContent: 'center' }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' }, purchaseToggle: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginHorizontal: 40, marginTop: 8 }, purchaseToggleCopy: { flex: 1 }, purchaseToggleTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12 }, purchaseToggleHint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginTop: 1 }, priceStatus: { backgroundColor: colors.surface, borderRadius: radii.sm, marginHorizontal: 36, marginTop: 7, paddingHorizontal: 11, paddingVertical: 8 }, priceStatusMatched: { alignItems: 'center', backgroundColor: colors.blueTint, flexDirection: 'row', gap: 8, justifyContent: 'space-between' }, priceStatusText: { color: colors.textMuted, flex: 1, fontFamily: fonts.medium, fontSize: 10 }, priceStatusMatchedText: { color: colors.blueDeep, fontFamily: fonts.bold }, priceStatusPrice: { color: colors.blue, fontFamily: fonts.bold, fontSize: 11 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' }, backHome: { color: colors.blue, fontFamily: fonts.bold, fontSize: 13, paddingVertical: 13, textAlign: 'center' }, reviewActions: { gap: 9 }, purchaseReview: { paddingHorizontal: 6 }, reviewKicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4, marginTop: 4 }, productCard: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.lg, flexDirection: 'row', gap: 15, marginTop: 14, padding: 14 }, productImage: { backgroundColor: colors.white, borderRadius: radii.md, height: 88, width: 88 }, productFallback: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: radii.md, height: 88, justifyContent: 'center', width: 88 }, productFallbackText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 30 }, productCopy: { flex: 1 }, productTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 17, lineHeight: 22 }, productMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 5 }, insightCard: { backgroundColor: colors.blueTint, borderRadius: radii.lg, marginTop: 14, padding: 18 }, insightSaving: { backgroundColor: colors.greenTint }, insightTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 20 }, insightCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, lineHeight: 21, marginTop: 7 }, reviewNote: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 11, lineHeight: 16, marginHorizontal: 8, marginTop: 13 }, reasonScreen: { flex: 1, justifyContent: 'space-between', paddingBottom: 20 }, warning: { color: colors.orange, fontFamily: fonts.bold, fontSize: 42, marginTop: 30, textAlign: 'center' }, reasonTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25, marginTop: 12, textAlign: 'center' }, reasonCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginHorizontal: 22, marginTop: 9, textAlign: 'center' }, reasonInput: { borderColor: colors.blue, borderRadius: radii.md, borderWidth: 1.5, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, marginTop: 22, minHeight: 110, padding: 13, textAlignVertical: 'top' } });
