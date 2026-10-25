#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Runs copies of the example consumer hooks in a fixture repo. A stub `docker`
# on PATH stands in for the real one: it appends its arguments to a log, one
# call per line, and exits STUB_FMT_EXIT for a `fmt` call and 0 otherwise, so
# no container ever starts.

function setup() {
  load test_helper/common
  make_repo
  mkdir .githooks
  cp -- "${REPO_DIR}/examples/consumer/.githooks/pre-commit" "${REPO_DIR}/examples/consumer/.githooks/pre-push" \
    .githooks/
  STUB_BIN="${BATS_TEST_TMPDIR}/bin"
  STUB_LOG="${BATS_TEST_TMPDIR}/docker.log"
  export STUB_LOG
  mkdir "${STUB_BIN}"
  cat > "${STUB_BIN}/docker" << 'STUB'
#!/usr/bin/env bash
printf 'docker' >> "${STUB_LOG}"
printf ' %s' "$@" >> "${STUB_LOG}"
printf '\n' >> "${STUB_LOG}"
for arg in "$@"; do
  if [[ "${arg}" == 'fmt' ]]; then
    exit "${STUB_FMT_EXIT:-0}"
  fi
done
exit 0
STUB
  chmod +x "${STUB_BIN}/docker"
  PATH="${STUB_BIN}:${PATH}"
  ROOT="$(git rev-parse --show-toplevel)"
  COMMON="$(git rev-parse --path-format=absolute --git-common-dir)"
  HOST_USER="$(id -u):$(id -g)"
  IMAGE="$(sed --quiet "s/^readonly IMAGE='\(.*\)'$/\1/p" .githooks/pre-commit)"
  # The docker options every hook's lint run passes, up to the runner's
  # arguments.
  LINT_RUN="docker run --rm --interactive --network=none --user ${HOST_USER} --volume ${ROOT}:${ROOT}:ro"
  LINT_RUN+=" --volume ${COMMON}:${COMMON}:ro --workdir ${ROOT} ${IMAGE} run"
  # The pre-commit fmt run for the formatter fx: the root mount is read-write.
  FMT_RUN="docker run --rm --network=none --user ${HOST_USER} --volume ${ROOT}:${ROOT}"
  FMT_RUN+=" --volume ${COMMON}:${COMMON}:ro --workdir ${ROOT} ${IMAGE} fmt --fail-on-change fx"
}

# @description List the formatter fx, and a commented-out go, in the copy of
#              the pre-commit hook's FORMATTERS block.
# @noargs
function enable_formatter() {
  sed --in-place 's/^readonly FORMATTERS=($/&\n  fx\n  # go/' .githooks/pre-commit
}

@test 'the shipped pre-commit example lists no formatter' {
  run sed --quiet '/^readonly FORMATTERS=($/,/^)$/p' .githooks/pre-commit
  assert_output 'readonly FORMATTERS=(
)'
}

@test 'pre-commit with an empty FORMATTERS block runs only the lints' {
  run .githooks/pre-commit
  assert_success
  run cat "${STUB_LOG}"
  assert_equal "${#lines[@]}" 1
  assert_line --index 0 --partial "${LINT_RUN} --hook pre-commit "
}

@test 'pre-commit with a formatter runs fmt read-write before the lints' {
  enable_formatter
  run .githooks/pre-commit
  assert_success
  run cat "${STUB_LOG}"
  assert_equal "${#lines[@]}" 2
  assert_line --index 0 "${FMT_RUN}"
  assert_line --index 1 --partial "${LINT_RUN} --hook pre-commit "
}

@test 'pre-commit stops with status 1 when fmt exits 1, before the lints' {
  enable_formatter
  STUB_FMT_EXIT=1 run .githooks/pre-commit
  assert_failure 1
  run cat "${STUB_LOG}"
  assert_output "${FMT_RUN}"
}

@test 'pre-commit stops with status 2 when fmt exits 2, before the lints' {
  enable_formatter
  STUB_FMT_EXIT=2 run .githooks/pre-commit
  assert_failure 2
  run cat "${STUB_LOG}"
  assert_output "${FMT_RUN}"
}

@test 'pre-push runs only the lints, with the remote, and never fmt' {
  run .githooks/pre-push origin 'https://example.invalid/repo.git'
  assert_success
  run cat "${STUB_LOG}"
  assert_equal "${#lines[@]}" 1
  assert_line --index 0 --partial "${LINT_RUN} --hook pre-push --remote origin "
}
