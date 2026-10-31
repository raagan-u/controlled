import { useEffect } from 'react';
import { useFonts } from 'expo-font';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

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
    <>
      <Stack screenOptions={{ animation: 'slide_from_right', animationDuration: 320, gestureEnabled: true, headerShown: false }}>
        <Stack.Screen name="index" options={{ animation: 'fade' }} />
        <Stack.Screen name="home" options={{ animation: 'fade', animationDuration: 380 }} />
        <Stack.Screen name="welcome" options={{ animation: 'fade' }} />
        <Stack.Screen name="hello" options={{ animation: 'fade', gestureEnabled: false }} />
      </Stack>
      <StatusBar style="dark" />
    </>
  );
}
