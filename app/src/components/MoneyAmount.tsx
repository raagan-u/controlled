import { StyleSheet, Text, View } from 'react-native';
import { colors, fonts } from '@/theme';
type Props = { caption?: string; tone?: 'default' | 'danger'; value: string };
export function MoneyAmount({ caption, tone = 'default', value }: Props) { return <View style={styles.wrap}><Text adjustsFontSizeToFit numberOfLines={1} style={[styles.value, tone === 'danger' && styles.danger]}>${value || '0'}</Text>{caption ? <Text style={styles.caption}>{caption}</Text> : null}</View>; }
const styles = StyleSheet.create({ wrap: { alignItems: 'center', minHeight: 120, justifyContent: 'center' }, value: { color: colors.navy, fontFamily: fonts.bold, fontSize: 60, letterSpacing: -2, maxWidth: '100%' }, danger: { color: colors.redText }, caption: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 13, marginTop: 4 } });
