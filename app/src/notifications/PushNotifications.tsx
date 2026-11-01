import { createContext, type PropsWithChildren, useCallback, useContext, useEffect, useMemo, useRef } from 'react';
import Constants from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import * as SecureStore from 'expo-secure-store';
import { router, type Href, usePathname } from 'expo-router';
import { AppState, Platform } from 'react-native';

import { registerPushDevice, unregisterPushDevice } from '@/api/notifications';
import { getSessionToken } from '@/auth/session';

type PushContextValue = { unregister: () => Promise<void> };
type NotificationData = { entityId?: unknown; type?: unknown };

const PushContext = createContext<PushContextValue | null>(null);
const pushTokenKey = 'controlled_expo_push_token';

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldPlaySound: true,
    shouldSetBadge: false,
    shouldShowBanner: true,
    shouldShowList: true,
  }),
});

function projectID() {
  const constants = Constants as typeof Constants & { easConfig?: { projectId?: string } };
  return process.env.EXPO_PUBLIC_EAS_PROJECT_ID ?? constants.easConfig?.projectId ?? Constants.expoConfig?.extra?.eas?.projectId;
}

function notificationRoute(data: NotificationData): Href | null {
  const id = typeof data.entityId === 'string' ? data.entityId : '';
  if (data.type === 'PAYMENT_REQUEST' && id) return `/requests/${id}` as Href;
  if (data.type === 'ACCOUNTABILITY_REQUEST' && id) return `/accountability/${id}` as Href;
  if (data.type === 'ACCOUNTABILITY_INVITATION') return '/invitations';
  return null;
}

function responseRoute(response: Notifications.NotificationResponse | null | undefined) {
  if (!response) return null;
  return notificationRoute(response.notification.request.content.data as NotificationData);
}

function isAuthenticatedPath(pathname: string) {
  return !['/', '/welcome', '/signin', '/signup', '/hello'].includes(pathname);
}

export function PushNotificationProvider({ children }: PropsWithChildren) {
  const pathname = usePathname();
  const token = useRef('');
  const registering = useRef(false);
  const pendingRoute = useRef<Href | null>(null);
  const authenticatedPath = useRef(false);

  const register = useCallback(async () => {
    if (registering.current || token.current || !Device.isDevice || Platform.OS === 'web' || !await getSessionToken()) return;
    const expoProjectID = projectID();
    if (!expoProjectID) return;
    registering.current = true;
    try {
      if (Platform.OS === 'android') {
        await Notifications.setNotificationChannelAsync('actions', {
          importance: 4,
          lightColor: '#1754EB',
          name: 'Requests and approvals',
          sound: 'default',
          vibrationPattern: [0, 220, 120, 220],
        });
      }
      let permission = await Notifications.getPermissionsAsync();
      if (permission.status !== 'granted') permission = await Notifications.requestPermissionsAsync();
      if (permission.status !== 'granted') return;
      const next = (await Notifications.getExpoPushTokenAsync({ projectId: expoProjectID })).data;
      await registerPushDevice(next, Platform.OS === 'ios' ? 'ios' : 'android');
      token.current = next;
      await SecureStore.setItemAsync(pushTokenKey, next);
    } catch {
      // Registration retries on the next navigation or app foreground.
    } finally {
      registering.current = false;
    }
  }, []);

  const unregister = useCallback(async () => {
    const current = token.current || await SecureStore.getItemAsync(pushTokenKey).catch(() => null) || '';
    token.current = '';
    try {
      if (current) await unregisterPushDevice(current);
    } finally {
      await SecureStore.deleteItemAsync(pushTokenKey).catch(() => undefined);
    }
  }, []);

  useEffect(() => {
    authenticatedPath.current = isAuthenticatedPath(pathname);
    const timer = setTimeout(() => void register(), 0);
    return () => clearTimeout(timer);
  }, [pathname, register]);

  useEffect(() => {
    const appStateSubscription = AppState.addEventListener('change', (state) => {
      if (state === 'active') void register();
    });
    return () => appStateSubscription.remove();
  }, [register]);

  useEffect(() => {
    const queueRoute = (response: Notifications.NotificationResponse | null | undefined) => {
      pendingRoute.current = responseRoute(response);
      if (pendingRoute.current && authenticatedPath.current) {
        const route = pendingRoute.current;
        pendingRoute.current = null;
        router.push(route);
      }
    };
    const responseSubscription = Notifications.addNotificationResponseReceivedListener(queueRoute);
    void Notifications.getLastNotificationResponseAsync().then(queueRoute).catch(() => undefined);
    return () => responseSubscription.remove();
  }, []);

  useEffect(() => {
    if (!pendingRoute.current || !isAuthenticatedPath(pathname)) return;
    const route = pendingRoute.current;
    pendingRoute.current = null;
    router.push(route);
  }, [pathname]);

  const value = useMemo(() => ({ unregister }), [unregister]);
  return <PushContext.Provider value={value}>{children}</PushContext.Provider>;
}

export function usePushNotifications() {
  const value = useContext(PushContext);
  if (!value) throw new Error('usePushNotifications must be used inside PushNotificationProvider.');
  return value;
}
