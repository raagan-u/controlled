import { Pressable, StyleSheet, Text, View } from 'react-native';
import { colors, fonts, radii } from '@/theme';
const keys = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '0', '⌫'];
type Props = { onChange: (value: string) => void; value: string };
export function AmountKeypad({ onChange, value }: Props) {
  const press = (key: string) => { if (key === '⌫') return onChange(value.slice(0, -1)); if (key === '.' && value.includes('.')) return; const next = `${value}${key}`; const decimals = next.split('.')[1] ?? ''; if (decimals.length <= 2 && next.length <= 9) onChange(next); };
  return <View style={styles.panel}>{keys.map((key) => <Pressable accessibilityLabel={key === '⌫' ? 'Delete' : key} key={key} onPress={() => press(key)} style={({ pressed }) => [styles.key, pressed && styles.pressed]}><Text style={key === '⌫' || key === '.' ? styles.operator : styles.number}>{key}</Text></Pressable>)}</View>;
}
const styles = StyleSheet.create({ panel: { backgroundColor: colors.surfaceAlt, borderRadius: radii.keypad, flexDirection: 'row', flexWrap: 'wrap', padding: 10 }, key: { alignItems: 'center', height: 56, justifyContent: 'center', width: '33.333%' }, pressed: { backgroundColor: colors.key, borderRadius: radii.pill }, number: { color: colors.navy, fontFamily: fonts.bold, fontSize: 26 }, operator: { color: colors.keyText, fontFamily: fonts.medium, fontSize: 26 } });
