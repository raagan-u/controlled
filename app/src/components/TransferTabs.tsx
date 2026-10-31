import { router, type Href } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { PressableScale } from '@/motion';
import { colors, fonts, radii } from '@/theme';
export function TransferTabs({ active }: { active: 'Request' | 'Send' }) { return <View style={styles.track}>{(['Send', 'Request'] as const).map((tab) => <PressableScale key={tab} onPress={() => router.replace((tab === 'Send' ? '/send?swap=1' : '/request-money?swap=1') as Href)} style={[styles.tab, active === tab && styles.active]}><Text style={[styles.label, active === tab && styles.activeLabel]}>{tab}</Text></PressableScale>)}</View>; }
const styles = StyleSheet.create({ track: { alignSelf: 'center', flexDirection: 'row', height: 44, width: 220 }, tab: { alignItems: 'center', borderRadius: radii.pill, flex: 1, justifyContent: 'center' }, active: { backgroundColor: colors.segActive }, label: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 16 }, activeLabel: { color: colors.navy } });
