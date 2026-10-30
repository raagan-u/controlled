import { Pressable, StyleSheet, Text, View } from 'react-native';
import { colors, fonts, radii } from '@/theme';
type Props = { active: 'Fund' | 'Withdraw'; onFund: () => void; onWithdraw: () => void };
export function FlowTabs({ active, onFund, onWithdraw }: Props) { return <View style={styles.track}>{(['Fund', 'Withdraw'] as const).map((tab) => <Pressable key={tab} onPress={tab === 'Fund' ? onFund : onWithdraw} style={[styles.tab, active === tab && styles.active]}><Text style={[styles.label, active === tab && styles.activeLabel]}>{tab}</Text></Pressable>)}</View>; }
const styles = StyleSheet.create({ track: { alignSelf: 'center', backgroundColor: colors.white, flexDirection: 'row', height: 44, width: 220 }, tab: { alignItems: 'center', borderRadius: radii.pill, flex: 1, justifyContent: 'center' }, active: { backgroundColor: colors.segActive }, label: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 16 }, activeLabel: { color: colors.navy } });
