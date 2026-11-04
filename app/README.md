# Controlled mobile app

Expo React Native client for the Controlled Android MVP. The project retains iOS development support, but production iOS hardening and Sign in with Apple remain roadmap work.

## Implemented screens

- Welcome, sign-up, sign-in, Google authentication, and first-time name setup
- Home balance, recent recipients, active/scheduled pact cards, and expandable activity
- Fund, send, request money, and withdraw flows
- Optional purchase-intent price checking
- Payment-request list and detail/actions
- Pact list, creation wizard, detail/progress, history, and Stakes & Recovery
- Accountability invitations, circle creation, request list, and approve/deny/cancel detail
- Transaction details with remarks and pact-approval metadata
- Authenticated navigation badges and Expo notification deep links

The Home screen refreshes wallet/activity/pact data on focus, app foreground, and every ten seconds while visible. This keeps remote approvals and time-based pact transitions current without manual pull-to-refresh.

Pact creation enforces a 24-hour minimum. Pacts lasting at least seven days are marked recovery-eligible; shorter valid pacts are clearly explained as unable to unlock Commitment Reserve.

## Requirements

- Node version compatible with `package.json` (`24` is the normal local choice)
- Android Studio/SDK or Xcode
- a running Controlled API
- a native development build for Google sign-in and notifications

Expo Go is not supported for the complete app because Google sign-in uses native modules.

## Install and run

```bash
cd app
nvm use
npm install
npm run android
```

For iOS:

```bash
npm run ios
```

After a native build is installed, start Metro independently with:

```bash
npm start
```

## API URL

Defaults:

- Android emulator: `http://10.0.2.2:8080`
- iOS simulator: `http://127.0.0.1:8080`

For a physical device or hosted backend:

```bash
EXPO_PUBLIC_API_URL=http://192.168.1.20:8080 npm start
```

The phone must be able to reach that host/port. `EXPO_PUBLIC_API_URL` is public app configuration; never place secrets in it.

## Authentication

Email/password and Google authentication exchange credentials for a 24-hour Controlled session. The bearer token is stored in Expo SecureStore.

Google configuration consists of:

- the Web client ID used by the app and backend token verifier;
- the iOS client ID/URL scheme in `app.json`; and
- the Android OAuth client/signing certificate configuration in Google Cloud/Firebase.

`autoSelectOnSignIn` is disabled, but Google may still offer the device's previously used account. Password reset and email verification are not implemented yet.

The native application identifiers are `com.fromh.controlled` on both platforms.

## Push notifications

Controlled uses Expo Notifications and Expo Push Service. After authentication, a physical device registers its Expo token with the API. Notification taps navigate to the matching payment request, accountability request, invitation list, or received transaction.

Android native setup:

1. Link the EAS project. The current `app.json` contains `extra.eas.projectId`.
2. Register Android package `com.fromh.controlled` in Firebase.
3. Keep `google-services.json` in the app root. Treat the Firebase service-account key as a secret and never commit it.
4. Upload the FCM V1 service-account credential through `eas credentials` for the relevant build profile.
5. Rebuild/reinstall the native app; a Metro reload cannot add native notification configuration.

The backend sends through Expo and optionally uses `CONTROLLED_EXPO_ACCESS_TOKEN` when enhanced push security is enabled. Push is best-effort; in-app badges and refreshed API state are authoritative.

## EAS builds

Build profiles in `eas.json`:

- `development` — development client, internal distribution
- `preview` — internal distribution
- `production` — store-oriented build with auto-incremented app version

Example:

```bash
npx eas-cli@latest build --platform android --profile development
```

## Checks

```bash
npm run typecheck
npm run lint
npx expo-doctor
```

## Generated native projects

The repository currently contains native Android/iOS projects produced for development. When app configuration or native plugins change, regenerate/rebuild carefully and review native diffs. Do not expect a JavaScript-only Metro reload to apply native configuration changes.

## Troubleshooting

### API requests fail on a physical phone

Use the development machine's LAN IP, not `localhost` or Android emulator alias `10.0.2.2`. Confirm the API listens on `:8080` and the host firewall permits the connection.

### Google reports a developer/configuration error

Verify the installed build's package/bundle ID, Android signing SHA certificate, Web client ID, iOS client ID, and URL scheme. Rebuild after native config changes.

### Google keeps offering one account

Google Credential Manager remembers device accounts. Add/select another Google account on the device for external-user testing. Clearing app storage provides a clean Controlled session, though the OS can still offer saved Google accounts.

### Notifications do not appear

Use a physical device, grant permission, verify the EAS project ID and FCM V1 credential, rebuild the app, then inspect API logs for push-device registration and Expo delivery.

### Price checking is unavailable

This is a backend feature. Configure `CHANNEL3_API_KEY` on the API and restart it. The send flow remains usable without price checking.
