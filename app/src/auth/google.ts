import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';
import {
  GoogleOneTapSignIn,
  isCancelledResponse,
  isErrorWithCode,
  isNoSavedCredentialFoundResponse,
  isSuccessResponse,
  statusCodes,
} from 'react-native-nitro-google-signin';

const webClientId =
  '376706277927-s4mest4okp2a4par7m6tl69b9ut3rl7c.apps.googleusercontent.com';
const iosClientId =
  '376706277927-p59kimeh6k4qbvfmekj2je0uqom151br.apps.googleusercontent.com';
const sessionKey = 'controlled_session';

const localAPIURL = Platform.OS === 'android' ? 'http://10.0.2.2:8080' : 'http://127.0.0.1:8080';
const apiURL = (process.env.EXPO_PUBLIC_API_URL ?? localAPIURL).replace(/\/$/, '');

GoogleOneTapSignIn.configure({
  autoSelectOnSignIn: false,
  iosClientId,
  offlineAccess: false,
  webClientId,
});

type GoogleAuthenticationResult = 'authenticated' | 'cancelled';

type SessionResponse = {
  sessionToken?: string;
};

type APIError = {
  error?: {
    message?: string;
  };
};

export async function authenticateWithGoogle(): Promise<GoogleAuthenticationResult> {
  try {
    await GoogleOneTapSignIn.checkPlayServices();

    let response = await GoogleOneTapSignIn.signIn();
    if (isNoSavedCredentialFoundResponse(response)) {
      response = await GoogleOneTapSignIn.createAccount();
    }
    if (isNoSavedCredentialFoundResponse(response)) {
      response = await GoogleOneTapSignIn.presentExplicitSignIn();
    }
    if (isCancelledResponse(response)) {
      return 'cancelled';
    }
    if (!isSuccessResponse(response) || !response.data.idToken) {
      throw new Error(`Google sign-in ended without an identity token (${response.type}).`);
    }

    const apiResponse = await fetch(`${apiURL}/v1/auth/google`, {
      body: JSON.stringify({ idToken: response.data.idToken }),
      headers: { 'Content-Type': 'application/json' },
      method: 'POST',
    });
    const payload = (await apiResponse.json()) as SessionResponse & APIError;
    if (!apiResponse.ok) {
      throw new Error(payload.error?.message ?? 'Controlled could not complete Google sign-in.');
    }
    if (!payload.sessionToken) {
      throw new Error('Controlled did not return a session token.');
    }

    await SecureStore.setItemAsync(sessionKey, payload.sessionToken);
    return 'authenticated';
  } catch (error) {
    if (isErrorWithCode(error) && error.code === statusCodes.SIGN_IN_CANCELLED) {
      return 'cancelled';
    }
    if (isErrorWithCode(error) && error.code === statusCodes.PLAY_SERVICES_NOT_AVAILABLE) {
      throw new Error('Google Play Services must be installed or updated to continue.');
    }
    if (isErrorWithCode(error) && error.code === statusCodes.DEVELOPER_ERROR) {
      throw new Error('Google sign-in is not configured for this app build.');
    }
    throw error;
  }
}
