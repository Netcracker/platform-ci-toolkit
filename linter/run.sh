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

# Remember DEFAULT_BRANCH when the caller set it (docker -e / CI / export).
# Env files below would otherwise overwrite it with the image default.
_had_default_branch=0
if [ -n "${DEFAULT_BRANCH+x}" ]; then
  _had_default_branch=1
  _default_branch_from_runtime="$DEFAULT_BRANCH"
fi

# Load baseline environment defaults.
if [ -f /etc/platform-ci-toolkit/super-linter.env ]; then
  set -a
  # shellcheck disable=SC1090
  . /etc/platform-ci-toolkit/super-linter.env
  set +a
fi

# Optional consumer overrides (only keys listed in the file change; rest stay).
if [ -f "$CUSTOM_DIR/super-linter.env" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$CUSTOM_DIR/super-linter.env"
  set +a
fi

if [ "$_had_default_branch" -eq 1 ]; then
  export DEFAULT_BRANCH="$_default_branch_from_runtime"
fi

# Super-Linter resolves rule files as "${GITHUB_WORKSPACE}/${LINTER_RULES_PATH}/<file>", so an
# absolute path would be appended to the workspace and never match. Climb out of the workspace with
# exactly as many "../" segments as /tmp/lint is deep (physical path) to land on WORK_DIR.
_depth=$(realpath /tmp/lint 2>/dev/null | tr -cd '/' | wc -c | tr -d ' ')
_up=""
for _ in $(seq 1 "${_depth:-32}"); do _up="${_up}../"; done
export LINTER_RULES_PATH="${_up}${WORK_DIR#/}"

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

# run_helm_lint discovers every Helm chart under /tmp/lint (any directory
# containing Chart.yaml), runs `helm lint` on it, then renders it with
# `helm template` and validates the rendered manifests with kubeconform.
#
# Dependency charts vendored by `helm dependency update` (a Chart.yaml nested
# inside another chart's own charts/ directory) are skipped, since they are
# already linted as part of their parent chart.
#
# kubeconform's default schema catalog does not know about a consumer's own
# CRDs. For every chart that ships its CRDs under <chart>/crds/, this
# generates a JSON schema per CRD with openapi2jsonschema and adds that
# chart's scratch directory as an extra -schema-location, so instances of
# the chart's own CRDs get structurally validated instead of just skipped.
# -ignore-missing-schemas stays on as a fallback for any other kind
# kubeconform still doesn't know
run_helm_lint() {
  if [ ! -d /tmp/lint ]; then
    echo "[platform-ci-toolkit] /tmp/lint not found; skipping helm-lint"
    return 0
  fi

  local -a chart_dirs
  # -L: in CI, /tmp/lint is a symlink to the real checkout (created by
  # prepare-ci-workspace or by the block at the top of this script). BusyBox
  # find (used in this image) does not follow a symlinked starting path
  # without -L, so without it this silently finds zero charts in CI while
  # still finding them locally, where /tmp/lint is a real bind mount.
  mapfile -t chart_dirs < <(find -L /tmp/lint -name Chart.yaml -exec dirname {} \; | sort)

  if [ ${#chart_dirs[@]} -eq 0 ]; then
    echo "[platform-ci-toolkit] No Helm charts (Chart.yaml) found under /tmp/lint; skipping helm-lint"
    return 0
  fi

  echo "[platform-ci-toolkit] Found ${#chart_dirs[@]} Helm chart(s):"
  printf '  %s\n' "${chart_dirs[@]}"

  local rc=0
  local chart_dir parent_dir grandparent_dir rendered_file err_file
  local crd_dir crd_file schema_dir
  local -a schema_location_args

  for chart_dir in "${chart_dirs[@]}"; do
    parent_dir="$(dirname "${chart_dir}")"
    grandparent_dir="$(dirname "${parent_dir}")"

    if [ "$(basename "${parent_dir}")" = "charts" ] && [ -f "${grandparent_dir}/Chart.yaml" ]; then
      echo "[platform-ci-toolkit] Skipping ${chart_dir} (vendored dependency of ${grandparent_dir})"
      continue
    fi

    echo ""
    echo "[platform-ci-toolkit] === helm lint: ${chart_dir} ==="
    # shellcheck disable=SC2086
    if ! helm lint ${HELM_LINT_EXTRA_ARGS:-} "${chart_dir}"; then
      rc=1
    fi

    # Generate a JSON schema for each CRD this chart ships under crds/.
    # openapi2jsonschema always writes to the current directory using a
    # fixed {kind}_{version}.json naming scheme, so it has to run from
    # inside a scratch directory, one per chart.
    schema_dir=""
    crd_dir="${chart_dir}/crds"
    if [ -d "${crd_dir}" ]; then
      schema_dir="$(mktemp -d)"
      while IFS= read -r crd_file; do
        if ! ( cd "${schema_dir}" && openapi2jsonschema "${crd_file}" >/dev/null ); then
          echo "[platform-ci-toolkit] WARNING: failed to generate a JSON schema from ${crd_file}"
        fi
      done < <(find -L "${crd_dir}" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' \))
    fi

    # value-profiles/ is the standard location for a chart's deployment
    # value profiles (one values file per profile, e.g. per cloud provider).
    # When present, every profile is rendered and validated separately,
    # When absent, fall back to a single run with default values.
    local -a value_profiles
    mapfile -t value_profiles < <(find -L "${chart_dir}/value-profiles" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' \) 2>/dev/null | sort)
    if [ ${#value_profiles[@]} -eq 0 ]; then
      value_profiles=("")
    fi

    local profile_file profile_label
    local -a profile_args
    for profile_file in "${value_profiles[@]}"; do
      if [ -n "${profile_file}" ]; then
        profile_label="value-profiles/$(basename "${profile_file}")"
        profile_args=(-f "${profile_file}")
      else
        profile_label="default values"
        profile_args=()
      fi

      echo ""
      echo "[platform-ci-toolkit] === helm template + kubeconform: ${chart_dir} (${profile_label}) ==="
      # kubeconform filters input files by extension; a plain mktemp file has
      # none, which makes it silently report "0 resource found in 0 file".
      rendered_file="$(mktemp --suffix=.yaml)"
      err_file="$(mktemp)"

      # shellcheck disable=SC2086
      if ! helm template "${chart_dir}" "${profile_args[@]}" ${HELM_TEMPLATE_EXTRA_ARGS:-} >"${rendered_file}" 2>"${err_file}"; then
        echo "[platform-ci-toolkit] helm template failed for ${chart_dir} (${profile_label}):"
        cat "${err_file}" >&2
        rm -f "${rendered_file}" "${err_file}"
        rc=1
        continue
      fi

      schema_location_args=(-schema-location default)
      if [ -n "${schema_dir}" ]; then
        schema_location_args+=(-schema-location "${schema_dir}/{{ .ResourceKind }}_{{ .ResourceAPIVersion }}.json")
      fi

      # shellcheck disable=SC2086
      if ! kubeconform -strict -summary "${schema_location_args[@]}" -ignore-missing-schemas ${KUBECONFORM_EXTRA_ARGS:-} "${rendered_file}"; then
        rc=1
      fi

      rm -f "${rendered_file}" "${err_file}"
    done

    [ -n "${schema_dir}" ] && rm -rf "${schema_dir}"
  done

  return "${rc}"
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
  helm-lint)
    run_helm_lint
    exit $?
    ;;
  all)
    run_super_linter
    super_linter_exit=$?
    run_skillcheck
    skillcheck_exit=$?
    run_helm_lint
    helm_lint_exit=$?
    if [ "$super_linter_exit" -ne 0 ] || [ "$skillcheck_exit" -ne 0 ] || [ "$helm_lint_exit" -ne 0 ]; then
      exit 1
    fi
    exit 0
    ;;
  *)
    echo "[platform-ci-toolkit] Unknown command: $command"
    echo "[platform-ci-toolkit] Usage: run.sh [super-linter|skillcheck|helm-lint|all]"
    exit 1
    ;;
esac
