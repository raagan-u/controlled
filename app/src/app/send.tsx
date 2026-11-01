import { router, type Href, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, StyleSheet, Switch, Text, TextInput, View } from 'react-native';
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
import { colors, fonts, radii } from '@/theme';
import { APIError } from '@/api/client';

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
	const [priceCheck, setPriceCheck] = useState<PriceCheck | null>(null);
	const [checkingPrice, setCheckingPrice] = useState(false);
	const [priceStatus, setPriceStatus] = useState('');
	const [priceModalVisible, setPriceModalVisible] = useState(false);
  const [available, setAvailable] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [reviewId, setReviewId] = useState('');
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const valid = amountMinor > 0 && !over && recipient !== null && (!isPurchase || remark.trim().length >= 3);
	const handleAmountChange = useCallback((value: string) => {
		setAmount(value); setPriceCheck(null); setPriceStatus('');
	}, []);

  useEffect(() => { void getWallet().then(({ wallet }) => setAvailable(wallet.balances.available.amountMinor)).catch((cause) => setError(apiErrorMessage(cause))); }, []);

	async function runPriceCheck() {
		const query = remark.trim();
		if (!isPurchase || query.length < 3 || amountMinor <= 0 || checkingPrice) return;
		setPriceCheck(null); setCheckingPrice(true); setPriceStatus('Checking current prices…');
		try {
			const result = await checkPurchasePrice(query, amountMinor);
			setPriceCheck(result);
			setPriceStatus(result.matched ? `Matched ${result.productTitle}` : result.message);
			if (result.matched) setPriceModalVisible(true);
		} catch {
			setPriceStatus('Price check unavailable — your payment still works.');
		} finally {
			setCheckingPrice(false);
		}
	}

  async function send() {
    if (!valid || submitting) return;
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
  if (step === 'reason') return <SheetScreen><View style={styles.reasonScreen}><View><Text style={styles.warning}>△</Text><Text style={styles.reasonTitle}>This goes over your pact</Text><Text style={styles.reasonCopy}>Explain why you need this transaction. Your accountability circle will see this message.</Text><TextInput maxLength={300} multiline onChangeText={setRemark} placeholder="What happened?" placeholderTextColor={colors.textMuted} style={styles.reasonInput} value={remark} /></View><SwipeConfirm disabled={!remark.trim() || submitting} label={submitting ? 'Creating request…' : 'Swipe right to ask'} onConfirm={() => void send()} /></View></SheetScreen>;
  if (step === 'recipient') return <SheetScreen><TransferTabs active="Send" /><RecipientPicker initialQuery={preset} onSelect={(user) => { setRecipient(user); setError(''); setStep('amount'); }} /></SheetScreen>;
  return <><SheetScreen><View style={styles.entry}><View><TransferTabs active="Send" /><Text style={styles.payee}>paying to <Text onPress={() => setStep('recipient')} style={styles.payeeStrong}>{recipient?.displayName}⌄</Text></Text></View><View style={styles.amountBlock}><MoneyAmount calculation={calculation} caption={`Available $${available === null ? '—' : formatUSD(available)}`} onCalculationPress={() => { if (calculation) { handleAmountChange(calculation); setDisplayAmount(calculation); setCalculation(null); } }} tone={over ? 'danger' : 'default'} value={displayAmount} /><TextInput maxLength={80} onChangeText={(value) => { setRemark(value); setPriceCheck(null); setPriceStatus(''); }} onSubmitEditing={() => void runPriceCheck()} placeholder={isPurchase ? 'Product name and model' : 'Add a remark'} placeholderTextColor={colors.textMuted} returnKeyType={isPurchase ? 'done' : 'default'} style={styles.remark} value={remark} /><View style={styles.purchaseToggle}><View style={styles.purchaseToggleCopy}><Text style={styles.purchaseToggleTitle}>This is a purchase</Text><Text style={styles.purchaseToggleHint}>Check the price before sending</Text></View><Switch onValueChange={(value) => { setIsPurchase(value); setPriceCheck(null); setPriceStatus(''); setPriceModalVisible(false); }} thumbColor={colors.white} trackColor={{ false: colors.grabber, true: colors.blue }} value={isPurchase} /></View>{isPurchase ? <Pressable accessibilityRole={priceCheck?.matched ? 'button' : undefined} accessibilityLabel={priceCheck?.matched ? 'Open price check' : undefined} disabled={!priceCheck?.matched} onPress={() => setPriceModalVisible(true)} style={({ pressed }) => [styles.priceStatus, priceCheck?.matched && styles.priceStatusMatched, pressed && styles.priceStatusPressed]}><Text numberOfLines={2} style={[styles.priceStatusText, priceCheck?.matched && styles.priceStatusMatchedText]}>{remark.trim().length < 3 ? 'Add an exact product name and model.' : checkingPrice ? 'Checking current prices…' : priceCheck?.matched ? priceCheck.verdict === 'better_price' ? 'Better price found' : 'Price check ready' : priceStatus || 'Tap Done on the keyboard to check prices.'}</Text>{priceCheck?.matched ? <Text style={styles.priceStatusPrice}>View  ›</Text> : null}</Pressable> : null}{over ? <Text style={styles.error}>More than your available balance.</Text> : error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting || (isPurchase && checkingPrice)} key={`${submitting}-${error}`} label={submitting ? 'Sending…' : isPurchase && checkingPrice ? 'Checking price…' : undefined} onConfirm={() => void send()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={handleAmountChange} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen><PurchasePriceModal intendedAmountMinor={amountMinor} onClose={() => setPriceModalVisible(false)} priceCheck={priceCheck} visible={priceModalVisible} /></>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, payee: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, payeeStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { flex: 1, justifyContent: 'center' }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' }, purchaseToggle: { alignItems: 'center', flexDirection: 'row', justifyContent: 'space-between', marginHorizontal: 40, marginTop: 8 }, purchaseToggleCopy: { flex: 1 }, purchaseToggleTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 12 }, purchaseToggleHint: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 10, marginTop: 1 }, priceStatus: { backgroundColor: colors.surface, borderRadius: radii.sm, marginHorizontal: 36, marginTop: 7, paddingHorizontal: 11, paddingVertical: 8 }, priceStatusMatched: { alignItems: 'center', backgroundColor: colors.blueTint, flexDirection: 'row', gap: 8, justifyContent: 'space-between' }, priceStatusPressed: { opacity: 0.72 }, priceStatusText: { color: colors.textMuted, flex: 1, fontFamily: fonts.medium, fontSize: 10 }, priceStatusMatchedText: { color: colors.blueDeep, fontFamily: fonts.bold }, priceStatusPrice: { color: colors.blue, fontFamily: fonts.bold, fontSize: 11 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, done: { backgroundColor: colors.blue, borderRadius: radii.md, color: colors.white, fontFamily: fonts.bold, fontSize: 17, overflow: 'hidden', paddingVertical: 18, textAlign: 'center' }, backHome: { color: colors.blue, fontFamily: fonts.bold, fontSize: 13, paddingVertical: 13, textAlign: 'center' }, reasonScreen: { flex: 1, justifyContent: 'space-between', paddingBottom: 20 }, warning: { color: colors.orange, fontFamily: fonts.bold, fontSize: 42, marginTop: 30, textAlign: 'center' }, reasonTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25, marginTop: 12, textAlign: 'center' }, reasonCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginHorizontal: 22, marginTop: 9, textAlign: 'center' }, reasonInput: { borderColor: colors.blue, borderRadius: radii.md, borderWidth: 1.5, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, marginTop: 22, minHeight: 110, padding: 13, textAlignVertical: 'top' } });
