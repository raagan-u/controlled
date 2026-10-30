import { useEffect, useRef, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors, fonts, radii } from '@/theme';

const keys = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '0', '⌫'];
const operators = ['+', '−', '×', '÷'] as const;
const operatorSet = new Set<string>(operators);
type Props = { onChange: (value: string) => void; onDisplayChange?: (value: string) => void; value: string };

function evaluate(expression: string): number | null {
  const tokens = expression.match(/\d+(?:\.\d+)?|[+−×÷]/g);
  if (!tokens || tokens.join('') !== expression || tokens.length % 2 === 0) return null;
  const values: number[] = [Number(tokens[0])];
  const additions: string[] = [];

  for (let index = 1; index < tokens.length; index += 2) {
    const operator = tokens[index];
    const operand = Number(tokens[index + 1]);
    if (!Number.isFinite(operand)) return null;
    if (operator === '×' || operator === '÷') {
      if (operator === '÷' && operand === 0) return null;
      const left = values.pop();
      if (left === undefined) return null;
      values.push(operator === '×' ? left * operand : left / operand);
    } else {
      additions.push(operator);
      values.push(operand);
    }
  }

  let result = values[0];
  for (let index = 0; index < additions.length; index += 1) {
    result = additions[index] === '+' ? result + values[index + 1] : result - values[index + 1];
  }
  return Number.isFinite(result) && result >= 0 ? result : null;
}

function formatResult(result: number) {
  return result.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1');
}

export function AmountKeypad({ onChange, onDisplayChange, value }: Props) {
  const [expression, setExpression] = useState(value);
  const [settled, setSettled] = useState(false);
  const lastEmitted = useRef(value);

  useEffect(() => {
    if (value !== lastEmitted.current) {
      setExpression(value);
      setSettled(false);
    }
  }, [value]);

  useEffect(() => {
    onDisplayChange?.(expression);
    const timer = setTimeout(() => {
      if (!expression) {
        lastEmitted.current = '';
        onChange('');
        onDisplayChange?.('');
        setSettled(true);
        return;
      }
      const result = evaluate(expression);
      if (result === null) return;
      const formatted = formatResult(result);
      lastEmitted.current = formatted;
      onChange(formatted);
      onDisplayChange?.(formatted);
      setSettled(true);
    }, 550);
    return () => clearTimeout(timer);
  }, [expression, onChange, onDisplayChange]);

  const pressNumber = (key: string) => {
    setExpression((current) => {
      const base = settled ? '' : current;
      const currentNumber = base.split(/[+−×÷]/).at(-1) ?? '';
      if (key === '.' && currentNumber.includes('.')) return current;
      if (key !== '.' && (currentNumber.split('.')[1]?.length ?? 0) >= 2) return current;
      const next = `${base}${key === '.' && base === '' ? '0.' : key}`;
      return next.length <= 18 ? next : current;
    });
    setSettled(false);
  };

  const pressOperator = (operator: string) => {
    setExpression((current) => {
      const resultBase = settled ? lastEmitted.current : current;
      if (!resultBase) return current;
      if (operatorSet.has(resultBase.at(-1) ?? '')) return `${resultBase.slice(0, -1)}${operator}`;
      return `${resultBase}${operator}`;
    });
    setSettled(false);
  };

  const press = (key: string) => {
    if (key === '⌫') {
      setExpression((current) => current.slice(0, -1));
      setSettled(false);
      return;
    }
    pressNumber(key);
  };

  return (
    <View>
      <View style={styles.operators}>
        {operators.map((operator) => (
          <Pressable key={operator} onPress={() => pressOperator(operator)} style={({ pressed }) => pressed && styles.operatorPressed}>
            <Text style={styles.math}>{operator}</Text>
          </Pressable>
        ))}
      </View>
      <View style={styles.panel}>
        {keys.map((key) => (
          <Pressable accessibilityLabel={key === '⌫' ? 'Delete' : key} key={key} onPress={() => press(key)} style={({ pressed }) => [styles.key, pressed && styles.pressed]}>
            <Text style={key === '⌫' || key === '.' ? styles.operator : styles.number}>{key}</Text>
          </Pressable>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  operators: { flexDirection: 'row', justifyContent: 'space-around', paddingBottom: 12, paddingHorizontal: 20 },
  math: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 27, textAlign: 'center', width: 48 },
  operatorPressed: { opacity: 0.45 },
  panel: { backgroundColor: colors.surfaceAlt, borderRadius: radii.keypad, flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginBottom: 10, marginHorizontal: 8, padding: 10 },
  key: { alignItems: 'center', backgroundColor: colors.key, borderRadius: radii.pill, flexGrow: 1, height: 56, justifyContent: 'center', width: '30%' },
  pressed: { opacity: 0.65, transform: [{ scale: 0.97 }] },
  number: { color: colors.keyText, fontFamily: fonts.medium, fontSize: 26 },
  operator: { color: colors.keyText, fontFamily: fonts.medium, fontSize: 23 },
});
