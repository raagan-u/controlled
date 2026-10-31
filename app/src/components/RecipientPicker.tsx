import { useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';

import { apiErrorMessage, getRecipients } from '@/api/wallet';
import type { User } from '@/api/types';
import { ContactRow } from '@/components/ContactRow';
import { colors, fonts, radii } from '@/theme';

type Props = {
  initialQuery?: string;
  onSelect: (user: User) => void;
};

function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || '@';
}

export function RecipientPicker({ initialQuery = '', onSelect }: Props) {
  const [query, setQuery] = useState(initialQuery);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    const timer = setTimeout(() => {
      setLoading(true);
      setError('');
      void getRecipients(query)
        .then(({ users: matches }) => {
          if (active) setUsers(matches);
        })
        .catch((cause) => {
          if (active) {
            setUsers([]);
            setError(apiErrorMessage(cause));
          }
        })
        .finally(() => {
          if (active) setLoading(false);
        });
    }, 300);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [query]);

  return (
    <View style={styles.wrap}>
      <TextInput
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setQuery}
        placeholder="ctrld ID, email, or name"
        placeholderTextColor={colors.textMuted}
        style={styles.search}
        value={query}
      />
      <Text style={styles.kicker}>{query.trim() ? 'MATCHING USERS —' : 'AVAILABLE USERS —'}</Text>
      {loading ? <ActivityIndicator color={colors.blue} style={styles.loading} /> : null}
      {!loading && error ? <Text style={styles.message}>{error}</Text> : null}
      {!loading && !error && users.length === 0 ? (
        <Text style={styles.message}>No registered ctrld user matches that search.</Text>
      ) : null}
      {!loading && !error && users.length > 0 ? (
        <ScrollView keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false} style={styles.list}>
          {users.map((user) => (
            <ContactRow
              initials={initials(user.displayName)}
              key={user.id}
              name={user.displayName}
              onPress={() => onSelect(user)}
              username={`${user.ctrldId} · ${user.email}`}
            />
          ))}
        </ScrollView>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { flex: 1 },
  search: {
    backgroundColor: colors.surface,
    borderRadius: radii.md,
    color: colors.navy,
    fontFamily: fonts.medium,
    fontSize: 15,
    height: 48,
    marginTop: 16,
    paddingHorizontal: 16,
  },
  kicker: {
    color: colors.textMuted,
    fontFamily: fonts.bold,
    fontSize: 10,
    letterSpacing: 1.5,
    marginTop: 22,
  },
  list: { flex: 1, marginTop: 4 },
  loading: { marginTop: 28 },
  message: {
    color: colors.textMuted,
    fontFamily: fonts.medium,
    fontSize: 13,
    marginTop: 24,
    textAlign: 'center',
  },
});
