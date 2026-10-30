import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { ActivityRow } from '@/components/ActivityRow';
import { SheetScreen } from '@/components/SheetScreen';
import { activities } from '@/data/demo';
import { colors, fonts } from '@/theme';
const sections = ['Today', 'Yesterday', 'Oct 27'] as const;
export default function ActivityScreen() { return <SheetScreen><ScrollView contentContainerStyle={styles.list} showsVerticalScrollIndicator={false}>{sections.map((section) => { const items = activities.filter((item) => item.section === section); return <View key={section} style={styles.section}><Text style={styles.kicker}>{section.toUpperCase()} —</Text>{items.map((item, index) => <ActivityRow detailed item={item} key={item.id} last={index === items.length - 1} />)}</View>; })}</ScrollView></SheetScreen>; }
const styles = StyleSheet.create({ list: { paddingBottom: 30 }, section: { marginBottom: 18 }, kicker: { color: colors.textMuted, fontFamily: fonts.bold, fontSize: 10, letterSpacing: 1.5, marginBottom: 5 } });
