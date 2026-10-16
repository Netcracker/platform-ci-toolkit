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

# Run golangci-lint for the Go modules that need linting. Super-Linter's
# GO_MODULES linter is disabled because it loads the whole repository at
# once and times out in multi-module repositories. The custom runner:
# - Respects FILTER_REGEX_INCLUDE, so make lint-module only lints the
#   requested module.
# - When no filter is set, only lints modules that contain changed .go files
#   compared to DEFAULT_BRANCH.
lint_go_modules() {
  local go_status=0
  local golangci_config="${WORK_DIR}/.golangci.yml"
  local include_pattern="${FILTER_REGEX_INCLUDE:-}"
  local default_branch="${DEFAULT_BRANCH:-master}"
  local custom_go_lint="${PLATFORM_CI_CUSTOM_GO_LINT:-true}"
  local modules_to_lint=()

  echo "[platform-ci-toolkit] Custom Go linter enabled: $custom_go_lint"

  if [ "$custom_go_lint" != "true" ]; then
    echo "[platform-ci-toolkit] Custom Go linter disabled; skipping."
    return 0
  fi

  if [ ! -f "$golangci_config" ]; then
    echo "[platform-ci-toolkit] No .golangci.yml found at $golangci_config; skipping custom Go lint."
    return 0
  fi

  # Build a list of module directories to lint.
  while IFS= read -r -d '' gomod; do
    local module_dir
    module_dir=$(dirname "$gomod")

    # FILTER_REGEX_INCLUDE is normally a file path regex (relative to the repo
    # root), while module_dir is an absolute path inside the container. Test a
    # representative file path under the module so directory-only filters such as
    # "services/foo/.*" still match the module itself.
    if [ -n "$include_pattern" ]; then
      local sample_file
      sample_file="${module_dir#/tmp/lint/}/.placeholder"
      if ! printf '%s\n' "$sample_file" | grep -qE "$include_pattern"; then
        continue
      fi
    fi

    modules_to_lint+=("$module_dir")
  done < <(find /tmp/lint -type d \( -path "*/vendor" -o -path "*/.git" \) -prune -o -name go.mod -print0)

  echo "[platform-ci-toolkit] Found ${#modules_to_lint[@]} Go module(s) to consider."

  # Without a file filter, narrow the list to modules with changed Go files.
  if [ -z "$include_pattern" ] && [ "${#modules_to_lint[@]}" -gt 0 ]; then
    local changed_go_files
    # Use "git diff <default_branch>" instead of "<default_branch>...HEAD" so
    # local runs include uncommitted and staged changes in the working tree.
    changed_go_files=$(cd /tmp/lint && git diff "${default_branch}" --name-only -- '*.go' 2>/dev/null || true)

    echo "[platform-ci-toolkit] Changed Go files: $(echo "$changed_go_files" | wc -l)"

    if [ -z "$changed_go_files" ]; then
      echo "[platform-ci-toolkit] No changed Go files; skipping custom Go lint."
      return 0
    fi

    local filtered_modules=()
    for module_dir in "${modules_to_lint[@]}"; do
      local rel_module_dir="${module_dir#/tmp/lint/}"
      if printf '%s\n' "$changed_go_files" | grep -q "^${rel_module_dir}/"; then
        filtered_modules+=("$module_dir")
      fi
    done
    modules_to_lint=("${filtered_modules[@]}")
  fi

  echo "[platform-ci-toolkit] Linting ${#modules_to_lint[@]} Go module(s)."

  for module_dir in "${modules_to_lint[@]}"; do
    echo "[platform-ci-toolkit] Linting Go module: $module_dir"
    if ! (cd "$module_dir" && golangci-lint run --timeout=15m --config "$golangci_config" ./...); then
      go_status=1
    fi
  done

  return $go_status
}

command="${1:-all}"

case "$command" in
  go)
    lint_go_modules
    exit $?
    ;;
  super-linter)
    run_super_linter
    exit $?
    ;;
  all)
    set +e
    run_super_linter
    super_linter_status=$?

    lint_go_modules
    go_status=$?
    set -e

    if [ "$super_linter_status" -ne 0 ] || [ "$go_status" -ne 0 ]; then
      exit 1
    fi
    exit 0
    ;;
  *)
    echo "[platform-ci-toolkit] Unknown command: $command"
    echo "[platform-ci-toolkit] Usage: run.sh [super-linter|go|all]"
    exit 1
    ;;
esac
