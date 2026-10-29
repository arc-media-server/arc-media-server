#!/usr/bin/env bash
# Installs what building the Android TV app needs on a Linux machine without Android Studio:
# a JDK (21), the Android command-line tools, platform-tools, the SDK platform and build tools.
# Usage: tools/setup-sdk.sh [sdk-dir]   (default: $ANDROID_HOME or ~/android-sdk)
# Afterwards: export ANDROID_HOME=<sdk-dir>, then ./gradlew assembleDebug.
set -euo pipefail

SDK="${1:-${ANDROID_HOME:-$HOME/android-sdk}}"
TOOLS_ZIP="commandlinetools-linux-16111833_latest.zip"

if ! command -v java >/dev/null 2>&1; then
  echo "Install JDK 21 first (e.g. sudo apt-get install openjdk-21-jdk-headless)." >&2
  exit 1
fi

mkdir -p "$SDK/cmdline-tools"
if [ ! -x "$SDK/cmdline-tools/latest/bin/sdkmanager" ]; then
  tmp="$(mktemp -d)"
  curl -fsSL -o "$tmp/tools.zip" "https://dl.google.com/android/repository/$TOOLS_ZIP"
  unzip -q "$tmp/tools.zip" -d "$tmp"
  mv "$tmp/cmdline-tools" "$SDK/cmdline-tools/latest"
  rm -rf "$tmp"
fi

yes | "$SDK/cmdline-tools/latest/bin/sdkmanager" --sdk_root="$SDK" --licenses >/dev/null
# compileSdk 37 (today's AndroidX libraries need it); build-tools as AGP 9.5 uses by default.
"$SDK/cmdline-tools/latest/bin/sdkmanager" --sdk_root="$SDK" \
  "platform-tools" "platforms;android-37.0" "build-tools;36.0.0"

echo "Android SDK ready in $SDK. Now: export ANDROID_HOME=$SDK"
