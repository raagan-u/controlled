import { router, type Href } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';
import type { Withdrawal } from '@/api/types';
import { apiErrorMessage, createWithdrawal, formatUSD, getCurrentUser, getWallet, reconcileWithdrawal, toMinorUnits } from '@/api/wallet';
import { ActionButton } from '@/components/ActionButton';
import { AmountKeypad } from '@/components/AmountKeypad';
import { FlowTabs } from '@/components/FlowTabs';
import { MoneyAmount } from '@/components/MoneyAmount';
import { ResultState } from '@/components/ResultState';
import { SheetScreen } from '@/components/SheetScreen';
import { SwipeConfirm } from '@/components/SwipeConfirm';
import { colors, fonts, radii } from '@/theme';
import { APIError } from '@/api/client';
import { useActiveRefresh } from '@/hooks/useActiveRefresh';
import { assessTransaction, type TransactionAssessment } from '@/api/ai';
import { TransactionGuidance } from '@/components/TransactionGuidance';
import { isMeaningfulRemark } from '@/utils/remark';

type Step = 'amount' | 'reason' | 'processing' | 'success' | 'failed' | 'review';

export default function WithdrawScreen() {
  const [amount, setAmount] = useState('');
  const [displayAmount, setDisplayAmount] = useState('');
  const [calculation, setCalculation] = useState<string | null>(null);
  const [available, setAvailable] = useState<number | null>(null);
  const [destination, setDestination] = useState('PayPal');
	const [destinationEmail, setDestinationEmail] = useState('');
  const [withdrawal, setWithdrawal] = useState<Withdrawal | null>(null);
  const [step, setStep] = useState<Step>('amount');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [reason, setReason] = useState('');
	const [assessment, setAssessment] = useState<TransactionAssessment | null>(null);
	const [assessmentLoading, setAssessmentLoading] = useState(false);
	const [guidanceStatus, setGuidanceStatus] = useState('');
	const assessmentRequestId = useRef(0);
	const lastAssessmentKey = useRef('');
  const [reviewId, setReviewId] = useState('');
  const amountMinor = toMinorUnits(amount);
  const over = available !== null && amountMinor > available;
  const meaningfulReason = isMeaningfulRemark(reason);
  const valid = amountMinor > 0 && !over && meaningfulReason;
	const changeAmount = useCallback((value: string) => { setAmount(value); setAssessment(null); setAssessmentLoading(false); setGuidanceStatus(''); lastAssessmentKey.current = ''; assessmentRequestId.current += 1; }, []);
	const runAssessment = useCallback(async () => {
		const remark = reason.trim();
		if (!isMeaningfulRemark(remark) || amountMinor <= 0 || over) return false;
		const key = `${amountMinor}\u0000${remark}`;
		if (lastAssessmentKey.current === key) return true;
		const requestId = ++assessmentRequestId.current;
		setAssessmentLoading(true); setGuidanceStatus('');
		try {
			const result = await assessTransaction(amountMinor, remark, false);
			if (requestId !== assessmentRequestId.current) return false;
			setAssessment(result); lastAssessmentKey.current = key;
			return true;
		} catch {
			if (requestId !== assessmentRequestId.current) return false;
			setAssessment(null); setGuidanceStatus('Guidance is unavailable — your withdrawal still works.'); lastAssessmentKey.current = key;
			return true;
		} finally {
			if (requestId === assessmentRequestId.current) setAssessmentLoading(false);
		}
	}, [amountMinor, over, reason]);

  const refreshWallet = useCallback(async () => { try { const [{ wallet }, { user }] = await Promise.all([getWallet(), getCurrentUser()]); setAvailable(wallet.balances.available.amountMinor); setDestinationEmail(user.email); setDestination(`PayPal · ${user.email}`); } catch (cause) { setError(apiErrorMessage(cause)); } }, []);
  useActiveRefresh(refreshWallet);

  async function withdraw() {
    if (!valid || submitting) return;
		if (lastAssessmentKey.current !== `${amountMinor}\u0000${reason.trim()}`) {
			await runAssessment();
			return;
		}
    setSubmitting(true); setError('');
    try {
      const result = await createWithdrawal(destinationEmail, amountMinor, reason.trim());
      if ('accountabilityRequest' in result) { setReviewId(result.accountabilityRequest.id); setStep('review'); }
      else { setWithdrawal(result.withdrawal); setStep('processing'); }
    } catch (cause) { if (cause instanceof APIError && cause.code === 'policy_denied' && !reason.trim()) setStep('reason'); else setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  async function checkStatus() {
    if (!withdrawal || submitting) return;
    setSubmitting(true); setError('');
    try {
      const settled = await reconcileWithdrawal(withdrawal.id);
      setWithdrawal(settled);
	  if (settled.status === 'COMPLETED') setStep('success');
	  else if (settled.status === 'FAILED') setStep('failed');
    } catch (cause) { setError(apiErrorMessage(cause)); }
    finally { setSubmitting(false); }
  }

  useEffect(() => {
	if (step !== 'processing' || !withdrawal) return;
	const timer = setInterval(() => { void checkStatus(); }, 3000);
	return () => clearInterval(timer);
	// Polling is keyed to the active payout rather than each updated response object.
	// eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, withdrawal?.id]);

  if (step === 'success') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="The payout completed and your Activity has been updated." title="Withdrawal complete" /></SheetScreen>;
  if (step === 'failed') return <SheetScreen footer={<ActionButton label="Back home" onPress={() => router.replace('/home')} />}><ResultState amount={amount} detail="The payout failed, so the reserved money was returned to your available balance." title="Withdrawal failed" tone="failed" /></SheetScreen>;
  if (step === 'review') return <SheetScreen footer={<><ActionButton label="View request" onPress={() => reviewId && router.replace(`/accountability/${reviewId}` as Href)} /><Text onPress={() => router.replace('/home')} style={styles.backHome}>Back home</Text></>}><ResultState amount={amount} detail="Nothing is reserved while you wait. Your assigned partner has six hours before it moves to the next person." title="Your transaction needs approval" tone="pending" /></SheetScreen>;
  if (step === 'reason') return <SheetScreen><View style={styles.reasonScreen}><View><Text style={styles.warning}>△</Text><Text style={styles.reasonTitle}>This goes over your pact</Text><Text style={styles.reasonCopy}>Explain why this withdrawal is necessary. Your accountability circle will see this message.</Text><TextInput maxLength={300} multiline onChangeText={setReason} placeholder="What happened?" placeholderTextColor={colors.textMuted} style={styles.reasonInput} value={reason} /></View><SwipeConfirm disabled={!reason.trim() || submitting} label={submitting ? 'Creating request…' : 'Swipe right to ask'} onConfirm={() => void withdraw()} /></View></SheetScreen>;
  if (step === 'processing') return <SheetScreen footer={<ActionButton disabled={submitting} label={submitting ? 'Checking…' : 'Check status'} onPress={() => void checkStatus()} />}><ResultState amount={amount} detail={error || 'The money is reserved while PayPal completes the payout.'} title="It’s on its way" tone="pending" /></SheetScreen>;
  return <SheetScreen><View style={styles.entry}><FlowTabs active="Withdraw" onFund={() => router.replace('/fund?swap=1' as Href)} onWithdraw={() => undefined} /><Text style={styles.destination}>move funds to <Text style={styles.destinationStrong}>{destination}⌄</Text></Text><View style={styles.amountBlock}><MoneyAmount calculation={calculation} onCalculationPress={() => { if (calculation) { changeAmount(calculation); setDisplayAmount(calculation); setCalculation(null); } }} tone={over ? 'danger' : 'default'} value={displayAmount} />{calculation ? null : <View style={styles.availableRow}><Text style={[styles.available, over && styles.over]}>{over ? 'More than available' : 'Available'}   ${available === null ? '—' : formatUSD(available)}</Text><Text onPress={() => available !== null && changeAmount((available / 100).toFixed(2))} style={styles.max}>Max</Text></View>}<TextInput maxLength={80} onBlur={() => void runAssessment()} onChangeText={(value) => { setReason(value); setAssessment(null); setAssessmentLoading(false); setGuidanceStatus(''); lastAssessmentKey.current = ''; assessmentRequestId.current += 1; }} placeholder="What is this withdrawal for?" placeholderTextColor={colors.textMuted} style={styles.remark} value={reason} /><Text style={styles.remarkHelp}>ⓘ Clear details help guidance understand your spending.</Text><Text style={styles.reserve}>Staked and reserved money can’t be withdrawn.</Text><TransactionGuidance assessment={assessment} loading={assessmentLoading} status={guidanceStatus} />{reason.trim() && !meaningfulReason ? <Text style={styles.error}>Use a meaningful remark instead of random characters.</Text> : error ? <Text style={styles.error}>{error}</Text> : null}<SwipeConfirm disabled={!valid || submitting || assessmentLoading} key={`${submitting}-${error}`} label={submitting ? 'Creating withdrawal…' : undefined} onConfirm={() => void withdraw()} /></View><AmountKeypad onCalculationChange={setCalculation} onChange={changeAmount} onDisplayChange={setDisplayAmount} value={amount} /></View></SheetScreen>;
}

const styles = StyleSheet.create({ entry: { flex: 1, justifyContent: 'space-between' }, destination: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 8, textAlign: 'center' }, destinationStrong: { color: colors.navy, fontFamily: fonts.bold }, amountBlock: { alignItems: 'center', flex: 1, justifyContent: 'center' }, availableRow: { alignItems: 'center', flexDirection: 'row', justifyContent: 'center', marginTop: -22 }, available: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14 }, over: { color: colors.redText }, max: { backgroundColor: colors.softButton, borderRadius: 8, color: colors.blue, fontFamily: fonts.bold, fontSize: 12, marginLeft: 10, overflow: 'hidden', paddingHorizontal: 10, paddingVertical: 6 }, remark: { backgroundColor: colors.surface, borderRadius: radii.md, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, height: 48, marginHorizontal: 36, marginTop: 10, paddingHorizontal: 15, textAlign: 'center' }, remarkHelp: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 9, marginHorizontal: 40, marginTop: 5, textAlign: 'center' }, reserve: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 10 }, error: { color: colors.redText, fontFamily: fonts.medium, fontSize: 12, marginHorizontal: 24, marginTop: 10, textAlign: 'center' }, backHome: { color: colors.blue, fontFamily: fonts.bold, fontSize: 13, paddingVertical: 13, textAlign: 'center' }, reasonScreen: { flex: 1, justifyContent: 'space-between', paddingBottom: 20 }, warning: { color: colors.orange, fontFamily: fonts.bold, fontSize: 42, marginTop: 30, textAlign: 'center' }, reasonTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 25, marginTop: 12, textAlign: 'center' }, reasonCopy: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, lineHeight: 19, marginHorizontal: 22, marginTop: 9, textAlign: 'center' }, reasonInput: { borderColor: colors.blue, borderRadius: radii.md, borderWidth: 1.5, color: colors.navy, fontFamily: fonts.medium, fontSize: 14, marginTop: 22, minHeight: 110, padding: 13, textAlignVertical: 'top' } });
