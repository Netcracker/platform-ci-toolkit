#!/bin/bash
set -euo pipefail

echo "[platform-ci-toolkit] Starting common linter run.sh"

BASE_DIR="/etc/platform-ci-toolkit/base-liners"
CUSTOM_DIR="/tmp/lint/prechecks/custom-liners"
WORK_DIR="/tmp/platform-ci-toolkit/base-liners"

mkdir -p "$WORK_DIR"

# Copy baseline linter configs into a writable work directory outside the
# mounted repository so the consumer's working tree is not modified.
cp -r "${BASE_DIR}/." "$WORK_DIR/"

# Overlay consumer-specific linter configs from the mounted repository.
if [ -d "$CUSTOM_DIR" ]; then
  shopt -s nullglob dotglob
  for f in "$CUSTOM_DIR"/*; do
    if [ -f "$f" ]; then
      cp -f "$f" "$WORK_DIR/"
    fi
  done
  shopt -u nullglob dotglob
fi

# Load baseline environment defaults.
if [ -f /etc/platform-ci-toolkit/super-linter.env ]; then
  set -a
  # shellcheck disable=SC1090
  . /etc/platform-ci-toolkit/super-linter.env
  set +a
fi

# Allow consumers to override environment variables.
if [ -f "$CUSTOM_DIR/super-linter.env" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$CUSTOM_DIR/super-linter.env"
  set +a
fi

export LINTER_RULES_PATH="$WORK_DIR"

warn_if_missing_go_proxy() {
  if [ -n "${GOPROXY:-}" ]; then
    return 0
  fi

  if [ -d /tmp/lint ] && find /tmp/lint -name go.mod -print -quit 2>/dev/null | grep -q .; then
    echo "[platform-ci-toolkit] WARNING: Go modules were detected but GOPROXY is not set."
    echo "[platform-ci-toolkit]          If these modules depend on private repositories, GO_MODULES may fail"
    echo "[platform-ci-toolkit]          to download dependencies. Set GOPROXY or configure Git authentication"
    echo "[platform-ci-toolkit]          before running the linter."
  fi
}

run_super_linter() {
  warn_if_missing_go_proxy
  set +e
  /action/lib/linter.sh
  return $?
}

command="${1:-all}"

case "$command" in
  super-linter|all)
    run_super_linter
    exit $?
    ;;
  *)
    echo "[platform-ci-toolkit] Unknown command: $command"
    echo "[platform-ci-toolkit] Usage: run.sh [super-linter|all]"
    exit 1
    ;;
esac
