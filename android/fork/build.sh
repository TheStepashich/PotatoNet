#!/usr/bin/env bash
#
# Build the EasyTier Android fork with custom DNS (pnet).
# Reproduces the verified build. Run from android/fork/.
#
# Requirements (checked below):
#   - Android SDK: platforms;android-34, build-tools;34.0.0, NDK r26 (26.0.10792818)
#   - JDK 21
#   - rustup + android targets (aarch64-linux-android armv7-linux-androideabi
#     i686-linux-android x86_64-linux-android)
#   - pnpm
#
# Env overrides:
#   ANDROID_HOME, NDK_HOME, JAVA_HOME, WORK (scratch dir), EASYTIER_REPO

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK="${WORK:-/tmp/opencode/easytier-fork-build}"
REPO="${EASYTIER_REPO:-$WORK/EasyTier}"
PATCH="$HERE/easytier-android-custom-dns-v2.4.5.patch"
VER=v2.4.5

# --- toolchain discovery ------------------------------------------------
ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
NDK_HOME="${NDK_HOME:-$ANDROID_HOME/ndk/26.0.10792818}"
JAVA_HOME="${JAVA_HOME:-/usr/lib/jvm/java-21-openjdk}"

for c in git curl unzip pnpm; do
  command -v "$c" >/dev/null || { echo "missing: $c"; exit 1; }
done
command -v rustup >/dev/null || { echo "missing: rustup (https://rustup.rs)"; exit 1; }
[ -d "$ANDROID_HOME/platforms/android-34" ] || { echo "missing SDK platform android-34"; exit 1; }
[ -d "$NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/sysroot" ] || { echo "missing NDK r26 at $NDK_HOME"; exit 1; }

export ANDROID_HOME NDK_HOME JAVA_HOME
export PATH="$HOME/.cargo/bin:$HOME/.npm-global/bin:$JAVA_HOME/bin:$PATH"

echo "==> ANDROID_HOME=$ANDROID_HOME"
echo "==> NDK_HOME=$NDK_HOME"
echo "==> JAVA_HOME=$JAVA_HOME"

# --- rust android targets ----------------------------------------------
for t in aarch64-linux-android armv7-linux-androideabi i686-linux-android x86_64-linux-android; do
  rustup target add "$t" >/dev/null 2>&1 || rustup target add "$t"
done

# --- fetch + patch -----------------------------------------------------
mkdir -p "$WORK"
if [ ! -d "$REPO/.git" ]; then
  echo "==> cloning EasyTier $VER"
  git clone --depth 1 --branch "$VER" https://github.com/EasyTier/EasyTier.git "$REPO"
fi
cd "$REPO"
git checkout -- . 2>/dev/null || true
git apply --check "$PATCH" >/dev/null 2>&1 || git apply --reverse "$PATCH" >/dev/null 2>&1 || true
git apply "$PATCH"
echo "==> patch applied"

# --- pnpm: allow build scripts (esbuild etc.) --------------------------
if ! grep -q "allowBuilds:" pnpm-workspace.yaml; then
  cat >> pnpm-workspace.yaml <<'EOF'
allowBuilds:
  esbuild: true
  unrs-resolver: true
  vue-demi: true
EOF
fi

echo "==> pnpm install"
pnpm install --reporter=append-only

# --- frontend build ----------------------------------------------------
echo "==> build frontend-lib"
( cd easytier-web/frontend-lib && ./node_modules/.bin/vite build )
echo "==> build gui frontend"
( cd easytier-gui && ./node_modules/.bin/vite build )

# --- bindgen needs the NDK sysroot (kcp-sys) ---------------------------
SYSROOT="$NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/sysroot"
export BINDGEN_EXTRA_CLANG_ARGS_aarch64_linux_android="--target=aarch64-linux-android --sysroot=$SYSROOT -I$SYSROOT/usr/include -I$SYSROOT/usr/include/aarch64-linux-android"
export BINDGEN_EXTRA_CLANG_ARGS_armv7_linux_androideabi="--target=armv7-linux-androideabi --sysroot=$SYSROOT -I$SYSROOT/usr/include -I$SYSROOT/usr/include/arm-linux-androideabi"
export BINDGEN_EXTRA_CLANG_ARGS_i686_linux_android="--target=i686-linux-android --sysroot=$SYSROOT -I$SYSROOT/usr/include -I$SYSROOT/usr/include/i686-linux-android"
export BINDGEN_EXTRA_CLANG_ARGS_x86_64_linux_android="--target=x86_64-linux-android --sysroot=$SYSROOT -I$SYSROOT/usr/include -I$SYSROOT/usr/include/x86_64-linux-android"

echo "==> tauri android build (this is the long step)"
cd easytier-gui
./node_modules/.bin/tauri android build --config '{"build":{"beforeBuildCommand":""}}'

APK=src-tauri/gen/android/app/build/outputs/apk/universal/release/app-universal-release.apk
AAB=src-tauri/gen/android/app/build/outputs/bundle/universalRelease/app-universal-release.aab
mkdir -p "$HERE/build"
cp "$APK" "$HERE/build/easytier-custom-dns-$VER-universal.apk"
cp "$AAB" "$HERE/build/easytier-custom-dns-$VER-universal.aab"
echo "==> DONE:"
ls -lh "$HERE/build"
