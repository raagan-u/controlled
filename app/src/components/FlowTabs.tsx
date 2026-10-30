import { Pressable, StyleSheet, Text, View } from 'react-native';
import { colors, fonts, radii } from '@/theme';
type Props = { active: 'Fund' | 'Withdraw'; onFund: () => void; onWithdraw: () => void };
export function FlowTabs({ active, onFund, onWithdraw }: Props) { return <View style={styles.track}>{(['Fund', 'Withdraw'] as const).map((tab) => <Pressable key={tab} onPress={tab === 'Fund' ? onFund : onWithdraw} style={[styles.tab, active === tab && styles.active]}><Text style={[styles.label, active === tab && styles.activeLabel]}>{tab}</Text></Pressable>)}</View>; }
const styles = StyleSheet.create({ track: { backgroundColor: colors.surface, borderRadius: radii.pill, flexDirection: 'row', height: 44, padding: 4 }, tab: { alignItems: 'center', borderRadius: radii.pill, flex: 1, justifyContent: 'center' }, active: { backgroundColor: colors.white }, label: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 15 }, activeLabel: { color: colors.navy } });
