import {
  GoogleOneTapSignIn,
  isCancelledResponse,
  isErrorWithCode,
  isNoSavedCredentialFoundResponse,
  isSuccessResponse,
  statusCodes,
} from 'react-native-nitro-google-signin';

import { createControlledSession } from '@/auth/session';

const webClientId =
  '376706277927-s4mest4okp2a4par7m6tl69b9ut3rl7c.apps.googleusercontent.com';
const iosClientId =
  '376706277927-p59kimeh6k4qbvfmekj2je0uqom151br.apps.googleusercontent.com';
GoogleOneTapSignIn.configure({
  autoSelectOnSignIn: false,
  iosClientId,
  offlineAccess: false,
  webClientId,
});

type GoogleAuthenticationResult = 'authenticated' | 'cancelled';

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

    await createControlledSession('/v1/auth/google', { idToken: response.data.idToken });
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
