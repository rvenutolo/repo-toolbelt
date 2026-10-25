#!/usr/bin/env bash

# A stub .ci/build-dist for tests of the scripts that run the built runner,
# loaded after test_helper/common:
#
#   load test_helper/built-runner
#
# write_build_dist_stub writes it into the current directory's .ci/. It records
# its arguments in CALLS and exits with BUILD_STATUS when that is set; otherwise
# it lays out a runner stub at bin/toolbelt. The runner stub records its
# arguments, whether TOOLBELT_LINTS_DIR and TOOLBELT_FORMATTERS_DIR point at the
# lints and formatters directories beside it, the directory it ran in and,
# when GIT_INDEX_FILE is set, its value, then exits with RUNNER_STATUS (0 when
# unset). Neither stub runs anything else.

# @description Write the build-dist stub to .ci/build-dist.
# @noargs
function write_build_dist_stub() {
  mkdir --parents .ci
  cat > .ci/build-dist << 'STUB'
#!/usr/bin/env bash
printf 'build-dist %s\n' "$*" >> "${CALLS}"
if [[ -n "${BUILD_STATUS:-}" ]]; then
  exit "${BUILD_STATUS}"
fi
mkdir --parents "$1/bin" "$1/lints" "$1/formatters"
cat > "$1/bin/toolbelt" << 'RUNNER'
#!/usr/bin/env bash
printf 'toolbelt %s\n' "$*" >> "${CALLS}"
if [[ "$(realpath "${TOOLBELT_LINTS_DIR:-}")" == "$(realpath "$(dirname "$0")/../lints")" ]]; then
  printf 'lints dir ok\n' >> "${CALLS}"
fi
if [[ "$(realpath "${TOOLBELT_FORMATTERS_DIR:-}")" == "$(realpath "$(dirname "$0")/../formatters")" ]]; then
  printf 'formatters dir ok\n' >> "${CALLS}"
fi
printf 'ran in %s\n' "$(pwd)" >> "${CALLS}"
if [[ -n "${GIT_INDEX_FILE:-}" ]]; then
  printf 'index %s\n' "${GIT_INDEX_FILE}" >> "${CALLS}"
fi
exit "${RUNNER_STATUS:-0}"
RUNNER
chmod +x "$1/bin/toolbelt"
STUB
  chmod +x .ci/build-dist
}

# @description Print the scratch directory the build-dist stub was handed the
#              dist directory in, from its line in CALLS.
# @noargs
# @stdout the scratch directory, or nothing when build-dist did not run
function built_scratch_dir() {
  sed --quiet 's|^build-dist \(.*\)/dist$|\1|p' "${CALLS}"
}
