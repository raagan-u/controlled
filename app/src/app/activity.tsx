import { ScrollView, StyleSheet, Text } from 'react-native';
import { ActivityRow } from '@/components/ActivityRow';
import { SheetScreen } from '@/components/SheetScreen';
import { activities } from '@/data/demo';
import { colors, fonts } from '@/theme';
export default function ActivityScreen() { return <SheetScreen><Text style={styles.title}>Activity</Text><Text style={styles.subtitle}>Your money, in one place.</Text><ScrollView contentContainerStyle={styles.list} showsVerticalScrollIndicator={false}>{activities.map((item) => <ActivityRow item={item} key={item.id} />)}</ScrollView></SheetScreen>; }
const styles = StyleSheet.create({ title: { color: colors.navy, fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6 }, subtitle: { color: colors.textMuted, fontFamily: fonts.medium, fontSize: 14, marginTop: 4 }, list: { paddingTop: 18 } });
