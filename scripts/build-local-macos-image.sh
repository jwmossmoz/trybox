#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKER_DIR="$ROOT/ci/macos/packer"

TARGET="${TRYBOX_TARGET:-macos15-arm64}"
IPSW="${TRYBOX_MACOS_IPSW:-}"
IMAGE_NAME="${TRYBOX_IMAGE_NAME:-}"
BUILD_VM="${TRYBOX_BUILD_VM:-}"
CPU="${TRYBOX_IMAGE_CPU:-8}"
MEMORY_MB="${TRYBOX_IMAGE_MEMORY_MB:-16384}"
DISK="${TRYBOX_IMAGE_DISK_GB:-200}"
DISPLAY="${TRYBOX_IMAGE_DISPLAY:-1920x1200}"
REPLACE=0
DELETE_ON_FAILURE="${TRYBOX_DELETE_BUILD_VM_ON_FAILURE:-0}"
HEADLESS="${TRYBOX_IMAGE_HEADLESS:-false}"

usage() {
  cat <<'EOF'
Usage: scripts/build-local-macos-image.sh [options]

Build a local Tart image for trybox from a fresh macOS IPSW with Packer.

Options:
  --target NAME        Trybox target name. Default: macos15-arm64
  --ipsw PATH|URL      macOS restore image. Default is inferred from --target.
  --image NAME         Final local Tart image name. Default is inferred from --target.
  --build-vm NAME      Temporary Packer VM name.
  --cpu N              VM CPUs. Default: 8
  --memory-mb N        VM memory in MB. Default: 16384
  --disk-gb N          VM disk size in GB. Default: 200
  --display SIZE       Display size. Default: 1920x1200
  --headless           Hide the VM window while Packer drives Setup Assistant.
  --replace            Replace the existing local target image after a good build.
  --delete-on-failure  Delete the temporary build VM if Packer fails.
  -h, --help           Show this help.

Environment variables mirror the option names:
TRYBOX_TARGET, TRYBOX_MACOS_IPSW, TRYBOX_IMAGE_NAME, TRYBOX_BUILD_VM,
TRYBOX_IMAGE_CPU, TRYBOX_IMAGE_MEMORY_MB, TRYBOX_IMAGE_DISK_GB,
TRYBOX_IMAGE_DISPLAY, TRYBOX_IMAGE_HEADLESS.
EOF
}

default_image_for_target() {
  case "$1" in
    macos12-*) echo "trybox-macos12-arm64-image" ;;
    macos13-*) echo "trybox-macos13-arm64-image" ;;
    macos14-*) echo "trybox-macos14-arm64-image" ;;
    macos15-*) echo "trybox-macos15-arm64-image" ;;
    macos26-*) echo "trybox-macos26-arm64-image" ;;
    *) return 1 ;;
  esac
}

default_ipsw_for_target() {
  case "$1" in
    macos12-*) echo "https://updates.cdn-apple.com/2022FallFCS/fullrestores/012-66032/8D8D90C6-A876-4FFF-BBF4-D158939B3841/UniversalMac_12.6.1_21G217_Restore.ipsw" ;;
    macos13-*) echo "https://updates.cdn-apple.com/2023FallFCS/fullrestores/042-55833/C0830847-A2F8-458F-B680-967991820931/UniversalMac_13.6_22G120_Restore.ipsw" ;;
    macos14-*) echo "https://updates.cdn-apple.com/2024SummerFCS/fullrestores/062-52859/932E0A8F-6644-4759-82DA-F8FA8DEA806A/UniversalMac_14.6.1_23G93_Restore.ipsw" ;;
    macos15-*) echo "https://updates.cdn-apple.com/2025SummerFCS/fullrestores/093-10809/CFD6DD38-DAF0-40DA-854F-31AAD1294C6F/UniversalMac_15.6.1_24G90_Restore.ipsw" ;;
    macos26-*) echo "https://updates.cdn-apple.com/2026WinterFCS/fullrestores/122-00766/062A6121-2ABE-45D7-BCB1-72B666B6D2C2/UniversalMac_26.4_25E246_Restore.ipsw" ;;
    *) return 1 ;;
  esac
}

die() {
  echo "error: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

vm_exists() {
  tart list --quiet | grep -Fxq "$1"
}

cleanup() {
  local code=$?

  if ((code != 0)) && [[ "$DELETE_ON_FAILURE" == "1" ]] && [[ -n "${BUILD_VM:-}" ]] && vm_exists "$BUILD_VM"; then
    tart stop "$BUILD_VM" --timeout 30 >/dev/null 2>&1 || true
    tart delete "$BUILD_VM" >/dev/null 2>&1 || true
  fi

  exit "$code"
}

while (($#)); do
  case "$1" in
    --target)
      TARGET="${2:?missing value for --target}"
      shift 2
      ;;
    --ipsw)
      IPSW="${2:?missing value for --ipsw}"
      shift 2
      ;;
    --image)
      IMAGE_NAME="${2:?missing value for --image}"
      shift 2
      ;;
    --build-vm)
      BUILD_VM="${2:?missing value for --build-vm}"
      shift 2
      ;;
    --cpu)
      CPU="${2:?missing value for --cpu}"
      shift 2
      ;;
    --memory-mb)
      MEMORY_MB="${2:?missing value for --memory-mb}"
      shift 2
      ;;
    --disk-gb)
      DISK="${2:?missing value for --disk-gb}"
      shift 2
      ;;
    --display)
      DISPLAY="${2:?missing value for --display}"
      shift 2
      ;;
    --headless)
      HEADLESS=true
      shift
      ;;
    --replace)
      REPLACE=1
      shift
      ;;
    --delete-on-failure)
      DELETE_ON_FAILURE=1
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

need packer
need tart

[[ -d "$PACKER_DIR" ]] || die "Packer directory does not exist: $PACKER_DIR"

if [[ -z "$IMAGE_NAME" ]]; then
  IMAGE_NAME="$(default_image_for_target "$TARGET")" || die "no default target image for target $TARGET; pass --image"
fi

if [[ -z "$IPSW" ]]; then
  IPSW="$(default_ipsw_for_target "$TARGET")" || die "no default IPSW for target $TARGET; pass --ipsw"
fi

if [[ -z "$BUILD_VM" ]]; then
  BUILD_VM="trybox-build-${TARGET}-$(date +%Y%m%d%H%M%S)"
fi

if vm_exists "$BUILD_VM"; then
  die "temporary build VM already exists: $BUILD_VM"
fi

if vm_exists "$IMAGE_NAME" && ((REPLACE == 0)); then
  die "target image already exists: $IMAGE_NAME; pass --replace to rebuild it"
fi

MEMORY_GB=$(((MEMORY_MB + 1023) / 1024))
trap cleanup EXIT

vars=(
  -var "vm_name=$BUILD_VM"
  -var "ipsw=$IPSW"
  -var "cpu_count=$CPU"
  -var "memory_gb=$MEMORY_GB"
  -var "disk_size_gb=$DISK"
  -var "display=$DISPLAY"
  -var "headless=$HEADLESS"
)

echo "target: $TARGET"
echo "IPSW: $IPSW"
echo "temporary build VM: $BUILD_VM"
echo "final image: $IMAGE_NAME"
echo "resources: ${CPU}cpu ${MEMORY_GB}GB ${DISK}GB $DISPLAY"

packer init "$PACKER_DIR"
packer validate "${vars[@]}" "$PACKER_DIR"
packer build "${vars[@]}" "$PACKER_DIR"

tart stop "$BUILD_VM" --timeout 60 >/dev/null 2>&1 || true

if vm_exists "$IMAGE_NAME"; then
  tart stop "$IMAGE_NAME" --timeout 30 >/dev/null 2>&1 || true
  tart delete "$IMAGE_NAME"
fi

tart rename "$BUILD_VM" "$IMAGE_NAME"
BUILD_VM=""

echo "built local trybox image: $IMAGE_NAME"
echo "try it with: TRYBOX_TARGET=$TARGET trybox run -- uname -a"
