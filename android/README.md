# GoTunnel Android Host

This directory contains the Android host app for the GoTunnel mobile client.

## What is included

- Foreground service shell for keeping the tunnel process alive
- Boot receiver for auto-start on device reboot
- Network recovery helper for reconnect/restart triggers
- Basic configuration screen for server address and token
- Notification channel and ongoing service notification
- A native bridge that loads the `gomobile` Go client binding from `app/libs/gotunnelmobile.aar`

## Current status

The Android shell expects the real Go client core to be bundled as `android/app/libs/gotunnelmobile.aar`.
Run `./scripts/build.sh android` (or `.\scripts\build.ps1 android`) after installing `gomobile` to generate and copy the AAR into place before building the APK.

## Open in Android Studio

Open the `android/` folder as a Gradle project. Android Studio can sync it directly and generate a wrapper if you want to build from the command line later.

## Notes

- The foreground service is marked as `dataSync` and starts in sticky mode.
- Auto-start is controlled by the saved configuration.
- Network restoration restarts the native Go client.
- The packaged AAR is generated from `github.com/gotunnel/mobile/gotunnelmobile` using `gomobile bind -javapkg com.gotunnel.mobilebind`.

## Release signing

Release APKs must be signed before installation or self-update will work.

The release workflow reads these GitHub secrets:

- `GOTUNNEL_ANDROID_KEYSTORE_B64`
- `GOTUNNEL_ANDROID_STORE_PASSWORD`
- `GOTUNNEL_ANDROID_KEY_ALIAS`
- `GOTUNNEL_ANDROID_KEY_PASSWORD`

The same secrets are also used by the general CI workflow. When they are present on a non-`pull_request` run, CI uploads a signed release APK instead of a debug APK.

Keep using the same signing key for every future release, otherwise installed Android clients will not be able to upgrade in place.
