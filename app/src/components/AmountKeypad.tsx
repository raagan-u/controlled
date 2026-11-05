import { useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { PressableScale } from '@/motion';
import { colors, fonts, radii } from '@/theme';

const keys = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '0', '⌫'];
const operators = ['+', '−', '×', '÷'] as const;
const operatorSet = new Set<string>(operators);
type Props = {
  compact?: boolean;
  onCalculationChange?: (value: string | null) => void;
  onChange: (value: string) => void;
  onDisplayChange?: (value: string) => void;
  value: string;
};

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

export function AmountKeypad({ compact = false, onCalculationChange, onChange, onDisplayChange, value }: Props) {
  const [expression, setExpression] = useState(value);
  const lastEmitted = useRef(value);

  useEffect(() => {
    if (value !== lastEmitted.current) {
      setExpression(value);
    }
  }, [value]);

  useEffect(() => {
    onDisplayChange?.(expression);
    onCalculationChange?.(null);
    const isCalculation = operatorSet.has(expression.match(/[+−×÷]/)?.[0] ?? '');
    if (isCalculation) {
      lastEmitted.current = '';
      onChange('');
    }
    if (!expression) {
      lastEmitted.current = '';
      onChange('');
      onDisplayChange?.('');
      return;
    }
    const result = evaluate(expression);
    if (result === null) return;
    const formatted = formatResult(result);
    if (isCalculation) {
      onCalculationChange?.(formatted);
      return;
    }
    lastEmitted.current = formatted;
    onChange(formatted);
    onDisplayChange?.(formatted);
  }, [expression, onCalculationChange, onChange, onDisplayChange]);

  const pressNumber = (key: string) => {
    setExpression((current) => {
      let base = current;
      const currentNumber = base.split(/[+−×÷]/).at(-1) ?? '';
      if (key === '.' && currentNumber.includes('.')) return current;
      if (key !== '.' && (currentNumber.split('.')[1]?.length ?? 0) >= 2) return current;
      if (key !== '.' && currentNumber === '0') base = base.slice(0, -1);
      const next = `${base}${key === '.' && base === '' ? '0.' : key}`;
      return next.length <= 18 ? next : current;
    });
  };

  const pressOperator = (operator: string) => {
    setExpression((current) => {
      const resultBase = current;
      if (!resultBase) return current;
      if (operatorSet.has(resultBase.at(-1) ?? '')) return `${resultBase.slice(0, -1)}${operator}`;
      return `${resultBase}${operator}`;
    });
  };

  const press = (key: string) => {
    if (key === '⌫') {
      setExpression((current) => current.slice(0, -1));
      return;
    }
    pressNumber(key);
  };

  return (
    <View>
      <View style={[styles.operators, compact && styles.operatorsCompact]}>
        {operators.map((operator) => (
          <PressableScale key={operator} onPress={() => pressOperator(operator)} scaleTo={0.8}>
            <Text style={styles.math}>{operator}</Text>
          </PressableScale>
        ))}
      </View>
      <View style={[styles.panel, compact && styles.panelCompact]}>
        {keys.map((key) => (
          <PressableScale accessibilityLabel={key === '⌫' ? 'Delete' : key} key={key} onPress={() => press(key)} scaleTo={0.9} style={[styles.key, compact && styles.keyCompact]}>
            <Text style={key === '⌫' || key === '.' ? styles.operator : styles.number}>{key}</Text>
          </PressableScale>
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  operators: { flexDirection: 'row', justifyContent: 'space-around', paddingBottom: 12, paddingHorizontal: 20 },
  operatorsCompact: { paddingBottom: 5 },
  math: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 27, textAlign: 'center', width: 48 },
  panel: { backgroundColor: colors.surfaceAlt, borderRadius: radii.keypad, flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginBottom: 10, marginHorizontal: 8, padding: 10 },
  panelCompact: { gap: 6, marginBottom: 4, padding: 7 },
  key: { alignItems: 'center', backgroundColor: colors.key, borderRadius: radii.pill, flexGrow: 1, height: 56, justifyContent: 'center', width: '30%' },
  keyCompact: { height: 44 },
  number: { color: colors.keyText, fontFamily: fonts.medium, fontSize: 26 },
  operator: { color: colors.keyText, fontFamily: fonts.medium, fontSize: 23 },
});
