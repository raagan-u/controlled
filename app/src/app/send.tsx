import { router, type Href, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Keyboard, Pressable, StyleSheet, Switch, Text, TextInput, View } from 'react-native';
import type { PriceCheck, User } from '@/api/types';
import { apiErrorMessage, checkPurchasePrice, createTransfer, formatUSD, getWallet, toMinorUnits } from '@/api/wallet';
import { AmountKeypad } from '@/components/AmountKeypad';
import { MoneyAmount } from '@/components/MoneyAmount';
import { PurchasePriceModal } from '@/components/PurchasePriceModal';
import { RecipientPicker } from '@/components/RecipientPicker';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { TransferTabs } from '@/components/TransferTabs';
import { ShakeOnError } from '@/motion';
import { colors, fonts, radii } from '@/theme';
import { APIError } from '@/api/client';
import { assessTransaction, type TransactionAssessment } from '@/api/ai';
import { useActiveRefresh } from '@/hooks/useActiveRefresh';
import { TransactionGuidance } from '@/components/TransactionGuidance';
import { isMeaningfulRemark } from '@/utils/remark';

type Step = 'recipient' | 'amount' | 'reason' | 'success' | 'review';

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
	const [assessment, setAssessment] = useState<TransactionAssessment | null>(null);
	const [assessmentLoading, setAssessmentLoading] = useState(false);
	const [guidanceStatus, setGuidanceStatus] = useState('');
	const [remarkError, setRemarkError] = useState('');
	const [priceCheck, setPriceCheck] = useState<PriceCheck | null>(null);
	const [checkingPrice, setCheckingPrice] = useState(false);
	const [priceStatus, setPriceStatus] = useState('');
	const [priceModalVisible, setPriceModalVisible] = useState(false);
  const [amountAttention, setAmountAttention] = useState(false);
  const [amountAttentionKey, setAmountAttentionKey] = useState(0);
  const amountAttentionTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const assessmentRequestId = useRef(0);
	const lastAssessmentKey = useRef('');
  const lastPriceCheckKey = useRef('');
  const priceCheckRequestId = useRef(0);
  const [available, setAvailable] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [reviewId, setReviewId] = useState('');
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const meaningfulRemark = isMeaningfulRemark(remark);
  const valid = amountMinor > 0 && !over && recipient !== null && meaningfulRemark;
	const handleAmountChange = useCallback((value: string) => {
		setAmount(value); setPriceCheck(null); setPriceStatus(''); setCheckingPrice(false); setAssessment(null); setAssessmentLoading(false); setGuidanceStatus('');
		lastAssessmentKey.current = ''; assessmentRequestId.current += 1;
		lastPriceCheckKey.current = ''; priceCheckRequestId.current += 1;
	}, []);
	const drawAttentionToAmount = useCallback(() => {
		if (amountMinor > 0) return;
		setAmountAttention(true);
		setAmountAttentionKey((key) => key + 1);
		if (amountAttentionTimer.current) clearTimeout(amountAttentionTimer.current);
		amountAttentionTimer.current = setTimeout(() => setAmountAttention(false), 650);
	}, [amountMinor]);
	const runAssessment = useCallback(async () => {
		const reason = remark.trim();
		if (!isMeaningfulRemark(reason) || amountMinor <= 0 || over) return false;
		const key = `${amountMinor}\u0000${isPurchase}\u0000${reason}`;
		if (lastAssessmentKey.current === key) return true;
		lastAssessmentKey.current = key;
		const requestId = ++assessmentRequestId.current;
		setAssessmentLoading(true); setGuidanceStatus(''); setRemarkError('');
		try {
			const result = await assessTransaction(amountMinor, reason, isPurchase);
			if (requestId !== assessmentRequestId.current) return false;
			setAssessment(result);
			return true;
		} catch (cause) {
			if (requestId !== assessmentRequestId.current) return false;
			setAssessment(null); lastAssessmentKey.current = key;
			if (cause instanceof APIError && cause.code === 'invalid_transaction_remark') {
				setRemarkError(cause.message); lastAssessmentKey.current = '';
				return false;
			}
			setGuidanceStatus('Guidance is unavailable — your payment still works.');
			return true;
		} finally {
			if (requestId === assessmentRequestId.current) setAssessmentLoading(false);
		}
	}, [amountMinor, isPurchase, over, remark]);
	useEffect(() => {
		if (!valid) return;
		const timer = setTimeout(() => void runAssessment(), 450);
		return () => clearTimeout(timer);
	}, [runAssessment, valid]);

  const refreshWallet = useCallback(async () => { try { const { wallet } = await getWallet(); setAvailable(wallet.balances.available.amountMinor); } catch (cause) { setError(apiErrorMessage(cause)); } }, []);
  useActiveRefresh(refreshWallet);
  useEffect(() => () => { if (amountAttentionTimer.current) clearTimeout(amountAttentionTimer.current); }, []);

	const runPriceCheck = useCallback(async (forcePurchase = false) => {
		const query = remark.trim();
		if ((!isPurchase && !forcePurchase) || query.length < 3 || amountMinor <= 0 || over) return;
		const checkKey = `${query}\u0000${amountMinor}`;
		if (lastPriceCheckKey.current === checkKey) return;
		lastPriceCheckKey.current = checkKey;
		const requestId = ++priceCheckRequestId.current;
		setPriceCheck(null); setCheckingPrice(true); setPriceStatus('Checking current prices…');
		try {
			const result = await checkPurchasePrice(query, amountMinor);
			if (requestId !== priceCheckRequestId.current) return;
			setPriceCheck(result);
			setPriceStatus(result.matched ? `Matched ${result.productTitle}` : result.message);
			if (result.matched) setPriceModalVisible(true);
		} catch {
			if (requestId !== priceCheckRequestId.current) return;
			lastPriceCheckKey.current = '';
			setPriceStatus('Price check unavailable — your payment still works.');
		} finally {
			if (requestId === priceCheckRequestId.current) setCheckingPrice(false);
		}
	}, [amountMinor, isPurchase, over, remark]);

	useEffect(() => {
		if (!isPurchase || remark.trim().length < 3 || amountMinor <= 0 || over) return;
		const timer = setTimeout(() => void runPriceCheck(), 250);
		return () => clearTimeout(timer);
	}, [amountMinor, isPurchase, over, remark, runPriceCheck]);

  async function send() {
    if (!valid || submitting) return;
		const currentAssessmentKey = `${amountMinor}\u0000${isPurchase}\u0000${remark.trim()}`;
		if (lastAssessmentKey.current !== currentAssessmentKey) {
			await runAssessment();
			return;
		}
    setSubmitting(true); setError('');
    try {
		  if (!recipient) return;
      const result = await createTransfer(recipient.ctrldId, amountMinor, remark.trim(), assessment?.id, isPurchase);
      if ('accountabilityRequest' in result) { setReviewId(result.accountabilityRequest.id); setStep('review'); } else setStep('success');
    } catch (cause) { if (cause instanceof APIError && cause.code === 'policy_denied' && !remark.trim()) setStep('reason'); else setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  if (step === 'success') return <SheetScreen footer={<Text onPress={() => router.replace('/home')} style={styles.done}>Done</Text>}><ResultState amount={amount} detail={`Sent to ${recipient?.displayName}${remark ? ` · ${remark}` : ''}`} title="Money sent" /></SheetScreen>;
  if (step === 'review') return <SheetScreen footer={<><Text onPress={() => reviewId && router.replace(`/accountability/${reviewId}` as Href)} style={styles.done}>View request</Text><Text onPress={() => router.replace('/home')} style={styles.backHome}>Back home</Text></>}><ResultState amount={amount} detail="Nothing is reserved while you wait. Your assigned partner has six hours before it moves to the next person." title="Your transaction needs approval" tone="pending" /></SheetScreen>;
  if (step === 'reason') return <SheetScreen><View style={styles.reasonScreen}><View><Text style={styles.warning}>△</Text><Text style={styles.reasonTitle}>This goes over your pact</Text><Text style={styles.reasonCopy}>Explain why you need this transaction. Your accountability circle will see this message.</Text><TextInput maxLength={300} multiline onChangeText={setRemark} placeholder="What happened?" placeholderTextColor={colors.textMuted} style={styles.reasonInput} value={remark} /></View><SwipeConfirm disabled={!remark.trim() || submitting} label={submitting ? 'Creating request…' : 'Swipe right to ask'} onConfirm={() => void send()} /></View></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><TransferTabs active="Send" /><RecipientPicker initialQuery={preset} onSelect={(user) => { setRecipient(user); setError(''); setStep('amount'); }} /></SheetScreen>;
  const expandedPurchasePrompt = isPurchase && !remark;
  return <><SheetScreen><View style={styles.entry}><View><TransferTabs active="Send" /><View style={styles.payee}><Text style={styles.payeeLabel}>paying to</Text><Pressable accessibilityRole="button" onPress={() => setStep('recipient')} style={styles.payeePicker}><Text numberOfLines={1} style={styles.payeeStrong}>{recipient?.displayName}</Text><Text style={styles.payeeChevron}>⌄</Text></Pressable></View></View><View style={styles.amountBlock}><ShakeOnError amplitude={10} error={amountAttentionKey ? String(amountAttentionKey) : undefined}><MoneyAmount calculation={calculation} caption={`Available $${available === null ? '—' : formatUSD(available)}`} onCalculationPress={() => { if (calculation) { handleAmountChange(calculation); setDisplayAmount(calculation); setCalculation(null); } }} tone={over || amountAttention ? 'danger' : 'default'} value={displayAmount} /></ShakeOnError><TextInput maxLength={80} multiline={expandedPurchasePrompt} numberOfLines={expandedPurchasePrompt ? 2 : 1} onChangeText={(value) => { if (isPurchase && !remark.trim() && value.trim()) drawAttentionToAmount(); setRemark(value); setRemarkError(''); setAssessment(null); setAssessmentLoading(false); setGuidanceStatus(''); lastAssessmentKey.current = ''; assessmentRequestId.current += 1; setPriceCheck(null); setPriceStatus(''); setCheckingPrice(false); lastPriceCheckKey.current = ''; priceCheckRequestId.current += 1; }} onSubmitEditing={() => { Keyboard.dismiss(); drawAttentionToAmount(); void runPriceCheck(); }} placeholder={isPurchase ? 'Name the product and why you need it' : 'What is this payment for?'} placeholderTextColor={colors.textMuted} returnKeyType={isPurchase ? 'done' : 'default'} style={[styles.remark, expandedPurchasePrompt && styles.remarkExpanded]} value={remark} /><Text style={styles.remarkHelp}>ⓘ Clear details help guidance understand your spending.</Text><View style={styles.purchaseToggle}><Text style={styles.purchaseToggleTitle}>This is a purchase</Text><Switch onValueChange={(value) => { setIsPurchase(value); setAssessment(null); setAssessmentLoading(false); setGuidanceStatus(''); lastAssessmentKey.current = ''; assessmentRequestId.current += 1; setPriceCheck(null); setPriceStatus(''); setCheckingPrice(false); setPriceModalVisible(false); lastPriceCheckKey.current = ''; priceCheckRequestId.current += 1; if (value && remark.trim()) drawAttentionToAmount(); }} thumbColor={colors.white} trackColor={{ false: colors.grabber, true: colors.blue }} value={isPurchase} /></View>{isPurchase && priceCheck?.matched ? <Pressable accessibilityLabel="Open price check" accessibilityRole="button" onPress={() => setPriceModalVisible(true)} style={({ pressed }) => [styles.priceStatus, styles.priceStatusMatched, pressed && styles.priceStatusPressed]}><Text numberOfLines={1} style={[styles.priceStatusText, styles.priceStatusMatchedText]}>{priceCheck.verdict === 'better_price' ? 'Better price found' : 'Price check ready'}</Text><Text style={styles.priceStatusPrice}>View  ›</Text></Pressable> : isPurchase && (checkingPrice || priceStatus) ? <Text numberOfLines={2} style={styles.priceStatusHint}>{checkingPrice ? 'Checking current prices…' : priceStatus}</Text> : null}<TransactionGuidance assessment={assessment} loading={assessmentLoading} status={guidanceStatus} />{remark.trim() && !meaningfulRemark ? <Text style={styles.error}>Use a meaningful remark instead of random characters.</Text> : remarkError ? <Text style={styles.error}>{remarkError}</Text> : over ? <Text style={styles.error}>More than your available balance.</Text> : error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting || assessmentLoading || (isPurchase && checkingPrice)} key={`${submitting}-${error}`} label={submitting ? 'Sending…' : undefined} onConfirm={() => void send()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={handleAmountChange} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen><PurchasePriceModal intendedAmountMinor={amountMinor} onClose={() => setPriceModalVisible(false)} priceCheck={priceCheck} visible={priceModalVisible} /></>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, payee: { alignItems: 'center', flexDirection: 'row', justifyContent: 'center', marginTop: 8 }, payeeLabel: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13 }, payeePicker: { alignItems: 'center', flexDirection: 'row', gap: 6, marginLeft: 4, maxWidth: '65%', paddingVertical: 2 }, payeeStrong: { color: colors.navy, flexShrink: 1, fontFamily: fonts.bold, fontSize: 13 }, payeeChevron: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16, lineHeight: 17, marginTop: -3 }, amountBlock: { flex: 1, justifyContent: 'center' }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center', textAlignVertical: 'center' }, remarkExpanded: { height: 68, paddingVertical: 10 }, remarkHelp: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 9, marginHorizontal: 40, marginTop: 5, textAlign: 'center' }, purchaseToggle: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginHorizontal: 40, marginTop: 8 }, purchaseToggleTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12 }, priceStatus: { backgroundColor: colors.surface, borderRadius: radii.sm, marginHorizontal: 36, marginTop: 7, paddingHorizontal: 11, paddingVertical: 8 }, priceStatusMatched: { alignItems: 'center', backgroundColor: colors.blueTint, flexDirection: 'row', gap: 8, justifyContent: 'space-between' }, priceStatusPressed: { opacity: 0.72 }, priceStatusText: { color: colors.textMuted, flex: 1, fontFamily: fonts.medium, fontSize: 10 }, priceStatusMatchedText: { color: colors.blueDeep, fontFamily: fonts.bold }, priceStatusPrice: { color: colors.blue, fontFamily: fonts.bold, fontSize: 11 }, priceStatusHint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginHorizontal: 40, marginTop: 7, textAlign: 'center' }, guidance: { backgroundColor: colors.blueTint, borderRadius: radii.sm, marginHorizontal: 36, marginTop: 8, paddingHorizontal: 11, paddingVertical: 9 }, guidanceWarning: { backgroundColor: colors.orangeTint }, guidanceEyebrow: { color: colors.blue, fontFamily: fonts.bold, fontSize: 8, letterSpacing: 0.7 }, guidanceTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12, marginTop: 4 }, guidanceMessage: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, lineHeight: 14, marginTop: 2 }, guidanceStatus: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginHorizontal: 40, marginTop: 7, textAlign: 'center' }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' }, backHome: { color: colors.blue, fontFamily: fonts.bold, fontSize: 13, paddingVertical: 13, textAlign: 'center' }, reasonScreen: { flex: 1, justifyContent: 'space-between', paddingBottom: 20 }, warning: { color: colors.orange, fontFamily: fonts.bold, fontSize: 42, marginTop: 30, textAlign: 'center' }, reasonTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25, marginTop: 12, textAlign: 'center' }, reasonCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginHorizontal: 22, marginTop: 9, textAlign: 'center' }, reasonInput: { borderColor: colors.blue, borderRadius: radii.md, borderWidth: 1.5, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, marginTop: 22, minHeight: 110, padding: 13, textAlignVertical: 'top' } });
