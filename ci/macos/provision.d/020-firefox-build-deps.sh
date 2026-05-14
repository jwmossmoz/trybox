#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -x /opt/homebrew/bin/brew ]]; then
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

if ! command -v brew >/dev/null 2>&1; then
  NONINTERACTIVE=1 CI=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

brew update

packages=(
  ccache
  cmake
  git
  git-lfs
  gnupg
  jq
  mercurial
  nasm
  node
  python@3.12
  rustup-init
  unzip
  watchman
  yasm
  zip
)

for package in "${packages[@]}"; do
  brew list --versions "$package" >/dev/null 2>&1 || brew install "$package"
done

git lfs install

if command -v xcodebuild >/dev/null 2>&1; then
  sudo xcodebuild -license accept >/dev/null 2>&1 || true
  sudo xcodebuild -runFirstLaunch >/dev/null 2>&1 || true
fi

if ! xcrun --sdk macosx --show-sdk-path >/dev/null 2>&1; then
  echo "warning: no macOS SDK is visible; use a Cirrus Xcode seed image or install Xcode before Firefox builds" >&2
fi
