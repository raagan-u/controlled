import { useEffect } from 'react';
import { useFonts } from 'expo-font';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { ActionCountProvider } from '@/notifications/ActionCounts';

// Switching between sibling tabs (Send/Request, Fund/Withdraw) swaps in place with no screen transition.
const swapScreens = ['send', 'request-money', 'fund', 'withdraw'];
const swapped = (params: object | undefined) => (params as { swap?: string } | undefined)?.swap === '1';

export default function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    'Satoshi-Regular': require('../../assets/fonts/Satoshi-Regular.otf'),
    'Satoshi-Medium': require('../../assets/fonts/Satoshi-Medium.otf'),
    'Satoshi-Bold': require('../../assets/fonts/Satoshi-Bold.otf'),
    'Satoshi-Black': require('../../assets/fonts/Satoshi-Black.otf'),
  });

  useEffect(() => {
    if (fontError) {
      throw fontError;
    }
  }, [fontError]);

  if (!fontsLoaded) {
    return null;
  }

  return (
    <ActionCountProvider>
      <Stack screenOptions={{ animation: 'slide_from_right', animationDuration: 320, gestureEnabled: true, headerShown: false }}>
        <Stack.Screen name="index" options={{ animation: 'fade' }} />
        <Stack.Screen name="home" options={{ animation: 'fade', animationDuration: 380 }} />
        <Stack.Screen name="welcome" options={{ animation: 'fade' }} />
        {swapScreens.map((name) => <Stack.Screen key={name} name={name} options={({ route }) => (swapped(route.params) ? { animation: 'none' } : {})} />)}
        <Stack.Screen name="hello" options={{ animation: 'fade', gestureEnabled: false }} />
      </Stack>
      <StatusBar style="dark" />
    </ActionCountProvider>
  );
}
