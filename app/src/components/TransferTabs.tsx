import { router, type Href } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { colors, fonts, radii } from '@/theme';
export function TransferTabs({ active }: { active: 'Request' | 'Send' }) { return <View style={styles.track}>{(['Send', 'Request'] as const).map((tab) => <Pressable key={tab} onPress={() => router.replace((tab === 'Send' ? '/send' : '/request-money') as Href)} style={[styles.tab, active === tab && styles.active]}><Text style={[styles.label, active === tab && styles.activeLabel]}>{tab}</Text></Pressable>)}</View>; }
const styles = StyleSheet.create({ track: { alignSelf: 'center', flexDirection: 'row', height: 44, width: 220 }, tab: { alignItems: 'center', borderRadius: radii.pill, flex: 1, justifyContent: 'center' }, active: { backgroundColor: colors.segActive }, label: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 16 }, activeLabel: { color: colors.navy } });
