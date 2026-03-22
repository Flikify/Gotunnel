# GoTunnel Android Host

This directory contains a minimal Android Studio / Gradle project skeleton for the GoTunnel Android host app.

## What is included

- Foreground service shell for keeping the tunnel process alive
- Boot receiver for auto-start on device reboot
- Network recovery helper for reconnect/restart triggers
- Basic configuration screen for server address and token
- Notification channel and ongoing service notification
- A stub bridge layer that can later be replaced with a gomobile/native Go core binding

## Current status

The Go tunnel core is not wired into Android yet. `GoTunnelBridge` returns a stub controller so the app structure can be developed independently from the Go runtime integration.

## Open in Android Studio

Open the `android/` folder as a Gradle project. Android Studio can sync it directly and generate a wrapper if you want to build from the command line later.

## Notes

- The foreground service is marked as `dataSync` and starts in sticky mode.
- Auto-start is controlled by the saved configuration.
- Network restoration currently triggers a restart hook in the stub controller.
- Replace the stub bridge with a native binding when the Go client core is exported for Android.
