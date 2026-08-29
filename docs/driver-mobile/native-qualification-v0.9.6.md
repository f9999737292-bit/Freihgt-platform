# Driver Mobile Native Qualification v0.9.6

Qualification branch: `feat/driver-mobile-native-qualification-v0.9.6`  
Baseline: `9e2c73fe4340e48257913b13765adf6486ddd166`

## Platform identifiers

| Field | Value |
|---|---|
| App path | `apps/driver-mobile` |
| Display name | Freight Driver |
| Android application ID | `com.freightplatform.driver.pilot` |
| iOS bundle ID | `com.freightplatform.driver.pilot` |
| Capacitor | 6.x |
| Ionic Vue | 8.x |

## Token storage classification

Native session persistence uses `@capacitor/preferences` with web `sessionStorage` fallback.

Classification: **PILOT_ACCEPTABLE**

Production follow-up: evaluate OS secure storage (Android Keystore / iOS Keychain) before public store release.

## Staging connectivity for native devices

Workstation SSH tunnel (`127.0.0.1:18080`) is valid for browser/dev only.

Native device/emulator options:

| Platform | Recommended method |
|---|---|
| Android emulator | `10.0.2.2:<host-forwarded-port>` to workstation tunnel, or approved LAN staging gateway |
| Android device | Controlled LAN reverse proxy / approved temporary HTTPS staging endpoint |
| iOS Simulator | Host loopback via Mac port-forward to approved staging gateway |
| iOS device | Approved temporary HTTPS staging endpoint with TLS; no permanent `:18080` exposure |

Do not hardcode Selectel IP or staging secrets into product source.

Build-time only:

```env
VITE_API_BASE_URL=<approved-staging-gateway-url>
VITE_PILOT_TENANT_ID=<pilot-tenant-uuid>
```

## Native runtime evidence required

Browser/PWA success does **not** certify Android or iOS.

Required evidence per platform:

- debug/simulator build artifact
- native shell launch
- login through native WebView
- delay/problem submission
- Control Tower visibility via backend verification

## CI added in v0.9.6

- `driver-mobile-check`: typecheck, lint, unit tests, web build, `cap sync android`
- `driver-mobile-android-build`: `assembleDebug`
- `driver-mobile-ios-build`: macOS `xcodebuild` simulator build (creates `ios/` if missing)

Native runtime E2E on physical devices remains an operator step outside GitHub CI.
