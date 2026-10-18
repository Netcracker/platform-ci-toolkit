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

run_super_linter() {
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
