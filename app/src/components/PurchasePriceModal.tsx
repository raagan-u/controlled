import { Image, Linking, Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import type { PriceCheck } from '@/api/types';
import { formatUSD } from '@/api/wallet';
import { colors, fonts, radii } from '@/theme';
import { ActionButton } from './ActionButton';

type Props = {
  intendedAmountMinor: number;
  onClose: () => void;
  priceCheck: PriceCheck | null;
  visible: boolean;
};

export function PurchasePriceModal({ intendedAmountMinor, onClose, priceCheck, visible }: Props) {
  if (!priceCheck?.matched) return null;
  const saving = priceCheck.verdict === 'better_price';
  const title = saving
    ? `You could save $${formatUSD(priceCheck.savingsMinor ?? 0)}`
    : priceCheck.verdict === 'below_market'
      ? 'Your price already looks strong'
      : 'Your price looks competitive';

  return (
    <Modal animationType="fade" onRequestClose={onClose} statusBarTranslucent transparent visible={visible}>
      <View style={styles.root}>
        <Pressable accessibilityLabel="Close price check" onPress={onClose} style={styles.scrim} />
        <View style={styles.sheet}>
          <View style={styles.handle} />
          <View style={styles.headingRow}>
            <View>
              <Text style={styles.kicker}>CTRLD PRICE CHECK —</Text>
              <Text style={styles.title}>{title}</Text>
            </View>
            <Pressable accessibilityLabel="Close" hitSlop={12} onPress={onClose}>
              <Text style={styles.close}>×</Text>
            </Pressable>
          </View>

          <View style={styles.product}>
            {priceCheck.imageUrl ? (
              <Image resizeMode="contain" source={{ uri: priceCheck.imageUrl }} style={styles.image} />
            ) : (
              <View style={styles.fallback}><Text style={styles.fallbackText}>{priceCheck.productTitle?.slice(0, 1) ?? 'P'}</Text></View>
            )}
            <View style={styles.productCopy}>
              <Text numberOfLines={3} style={styles.productTitle}>{priceCheck.productTitle}</Text>
              <Text style={styles.productMeta}>{[priceCheck.brand, priceCheck.merchant].filter(Boolean).join(' · ')}</Text>
            </View>
          </View>

          <View style={styles.comparison}>
            <View><Text style={styles.priceLabel}>YOU’RE PAYING</Text><Text style={styles.intentPrice}>${formatUSD(intendedAmountMinor)}</Text></View>
            <Text style={styles.arrow}>→</Text>
            <View style={styles.bestPrice}><Text style={styles.priceLabel}>BEST FOUND</Text><Text style={styles.offerPrice}>${formatUSD(priceCheck.bestPriceMinor ?? 0)}</Text></View>
          </View>

          <Text style={styles.context}>{priceCheck.message} Prices and availability can change, so confirm the product and retailer before switching.</Text>
          <View style={styles.actions}>
            {priceCheck.offerUrl ? <ActionButton label={`View offer at ${priceCheck.merchant}`} onPress={() => void Linking.openURL(priceCheck.offerUrl!)} /> : null}
            <ActionButton label="Ignore and continue" onPress={onClose} tone="soft" />
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, justifyContent: 'flex-end' },
  scrim: { backgroundColor: colors.scrim, bottom: 0, left: 0, position: 'absolute', right: 0, top: 0 },
  sheet: { backgroundColor: colors.white, borderTopLeftRadius: 28, borderTopRightRadius: 28, paddingBottom: 22, paddingHorizontal: 20, paddingTop: 10 },
  handle: { alignSelf: 'center', backgroundColor: colors.grabber, borderRadius: 3, height: 4, marginBottom: 18, width: 36 },
  headingRow: { alignItems: 'flex-start', flexDirection: 'row', justifyContent: 'space-between' },
  kicker: { color: colors.blue, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.4 },
  title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 24, letterSpacing: -0.4, marginTop: 5 },
  close: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 30, lineHeight: 30 },
  product: { alignItems: 'center', backgroundColor: colors.surface, borderRadius: radii.lg, flexDirection: 'row', gap: 14, marginTop: 18, padding: 13 },
  image: { backgroundColor: colors.white, borderRadius: radii.md, height: 78, width: 78 },
  fallback: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: radii.md, height: 78, justifyContent: 'center', width: 78 },
  fallbackText: { color: colors.blue, fontFamily: fonts.bold, fontSize: 28 },
  productCopy: { flex: 1 },
  productTitle: { color: colors.navy, fontFamily: fonts.bold, fontSize: 16, lineHeight: 21 },
  productMeta: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, marginTop: 5 },
  comparison: { alignItems: 'center', backgroundColor: colors.blueTint, borderRadius: radii.lg, flexDirection: 'row', justifyContent: 'space-between', marginTop: 12, padding: 16 },
  priceLabel: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 9, letterSpacing: 1 },
  intentPrice: { color: colors.navy, fontFamily: fonts.bold, fontSize: 22, marginTop: 3 },
  arrow: { color: colors.blue, fontFamily: fonts.medium, fontSize: 22 },
  bestPrice: { alignItems: 'flex-end' },
  offerPrice: { color: colors.greenText, fontFamily: fonts.bold, fontSize: 22, marginTop: 3 },
  context: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 12, lineHeight: 18, marginHorizontal: 4, marginTop: 12 },
  actions: { gap: 9, marginTop: 18 },
});
