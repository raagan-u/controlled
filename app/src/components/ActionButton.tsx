import { StyleSheet, Text } from 'react-native';
import { PressableScale } from '@/motion';
import { colors, fonts, radii } from '@/theme';
type Props = { disabled?: boolean; label: string; onPress: () => void; tone?: 'primary' | 'soft' | 'danger' };
export function ActionButton({ disabled, label, onPress, tone = 'primary' }: Props) { return <PressableScale scaleTo={0.97} accessibilityRole="button" accessibilityState={{ disabled }} disabled={disabled} onPress={onPress} style={[styles.button, styles[tone], disabled && styles.disabled]}><Text style={[styles.label, tone !== 'primary' && styles.darkLabel, tone === 'danger' && styles.dangerLabel]}>{label}</Text></PressableScale>; }
const styles = StyleSheet.create({ button: { alignItems: 'center', borderRadius: radii.md, height: 56, justifyContent: 'center', width: '100%' }, primary: { backgroundColor: colors.blue }, soft: { backgroundColor: colors.softButton }, danger: { backgroundColor: colors.redTint }, label: { color: colors.white, fontFamily: fonts.bold, fontSize: 17 }, darkLabel: { color: colors.navy }, dangerLabel: { color: colors.redText }, disabled: { opacity: 0.42 } });
