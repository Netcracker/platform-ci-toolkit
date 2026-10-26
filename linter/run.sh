#!/bin/bash
set -euo pipefail

echo "[platform-ci-toolkit] Starting common linter run.sh"

# CI runners (GitLab, GitHub Actions) check out the repository to a path like
# /builds/... or /__w/... rather than /tmp/lint. Link the expected workspace
# path to the actual checkout so Super-Linter and skillcheck find the code.
if [ ! -e /tmp/lint ]; then
  repo_dir=""
  if [ -n "${CI_PROJECT_DIR:-}" ]; then
    repo_dir="$CI_PROJECT_DIR"
  elif [ -n "${GITHUB_WORKSPACE:-}" ]; then
    repo_dir="$GITHUB_WORKSPACE"
  fi
  if [ -n "$repo_dir" ]; then
    echo "[platform-ci-toolkit] Linking /tmp/lint -> $repo_dir"
    ln -s "$repo_dir" /tmp/lint
  fi
fi

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

# run_skillcheck validates Agent Skills in /tmp/lint when SKILLCHECK_PATTERNS is
# configured. Patterns are shell globs relative to /tmp/lint. Only directories that
# actually contain SKILL.md (or a direct subdirectory with SKILL.md) are passed
# to skillcheck; everything else is silently ignored so that a generic pattern
# like "*" does not fail the lint job when no skills are present.
run_skillcheck() {
  if [ -z "${SKILLCHECK_PATTERNS:-}" ]; then
    echo "[platform-ci-toolkit] SKILLCHECK_PATTERNS not set; skipping skillcheck"
    return 0
  fi

  if [ ! -d /tmp/lint ]; then
    echo "[platform-ci-toolkit] /tmp/lint not found; skipping skillcheck"
    return 0
  fi

  local targets=()
  local dir

  for pattern in $SKILLCHECK_PATTERNS; do
    for dir in /tmp/lint/$pattern; do
      [ -d "$dir" ] || continue
      if [ -f "$dir/SKILL.md" ] || find "$dir" -maxdepth 2 -mindepth 2 -name SKILL.md -print -quit | grep -q .; then
        targets+=("$dir")
      fi
    done
  done

  if [ ${#targets[@]} -eq 0 ]; then
    echo "[platform-ci-toolkit] No skill folders match SKILLCHECK_PATTERNS ($SKILLCHECK_PATTERNS); skipping skillcheck"
    return 0
  fi

  echo "[platform-ci-toolkit] Running skillcheck on ${#targets[@]} folder(s):"
  printf '  %s\n' "${targets[@]}"

  set +e
  skillcheck "${targets[@]}"
  return $?
}

command="${1:-all}"

case "$command" in
  super-linter)
    run_super_linter
    exit $?
    ;;
  skillcheck)
    run_skillcheck
    exit $?
    ;;
  all)
    run_super_linter
    super_linter_exit=$?
    run_skillcheck
    skillcheck_exit=$?
    if [ "$super_linter_exit" -ne 0 ] || [ "$skillcheck_exit" -ne 0 ]; then
      exit 1
    fi
    exit 0
    ;;
  *)
    echo "[platform-ci-toolkit] Unknown command: $command"
    echo "[platform-ci-toolkit] Usage: run.sh [super-linter|skillcheck|all]"
    exit 1
    ;;
esac
