#!/usr/bin/env bats

# .ci/in-devshell: it must run the command in the devShell of the repo it lives
# in, return the command's status, and leave nothing behind in the caller's
# TMPDIR however the command ends.
#
# The first cases put a stub `nix` first on PATH, which records the command
# line the wrapper builds and runs nothing. The rest run the real
# `nix develop`, so they enter this repo's devShell. Every case that reaches
# the wrapper's nix path unsets IN_DEVSHELL. Payloads are inert (`true`,
# `sh -c 'exit 3'`, `sleep`, marker files), and signals go only to processes a
# case started, by recorded pid or by the process group of a job it started.
#
# common.bash points XDG_CONFIG_HOME at a path that does not exist, so nix
# reads no user nix.conf: a real run names the experimental features it needs
# in NIX_CONFIG.
#
# Background jobs redirect every descriptor, fd 3 included: bats waits for
# fd 3 to close, so a job holding it would stall the suite.
#
# setup sets WRAPPER, the script under test, SCRATCH, an empty directory a
# case hands the wrapper as its TMPDIR, STUB_DIR, the directory holding the
# stub nix, and STARTED, the file of pids a case has started and not yet seen
# end.

function setup() {
  load test_helper/common
  WRAPPER="${REPO_DIR}/.ci/in-devshell"
  SCRATCH="${BATS_TEST_TMPDIR}/tmp"
  mkdir -- "${SCRATCH}"
  STARTED="${BATS_TEST_TMPDIR}/started-pids"
  : > "${STARTED}"
  STUB_DIR="${BATS_TEST_TMPDIR}/stub"
  mkdir -- "${STUB_DIR}"
  cat > "${STUB_DIR}/nix" << 'STUB'
#!/usr/bin/env bash
printf 'nix-ran %s\n' "$*"
printf 'IN_DEVSHELL=%s\n' "${IN_DEVSHELL}"
printf 'TMPDIR=%s\n' "${TMPDIR}"
STUB
  chmod +x -- "${STUB_DIR}/nix"
}

# A case that fails midway must not leave a process it started running. A case
# records each pid it starts in the STARTED file and drops it once it has seen
# the process end: the kernel may hand that pid to an unrelated process, which
# a KILL here would then hit. A file, not an array: each case runs in a
# subshell of its own, so an array it extends would not reach teardown.
function teardown() {
  local pid
  while IFS= read -r pid; do
    kill -s KILL "${pid}" 2> '/dev/null' || true
  done < "${STARTED}"
}

# How long a case waits for a process to start or end, in tenths of a second.
# Entering the devShell may build or fetch it on a cold store.
readonly WAIT_TICKS=1200

# The KEEP_VARS allowlist, in the wrapper's order.
readonly KEEP_VARS=(
  HOME TERM CI IN_DEVSHELL NIX_SSL_CERT_FILE TMPDIR
  GITHUB_ACTIONS GITHUB_OUTPUT GITHUB_ENV GITHUB_STEP_SUMMARY GITHUB_WORKSPACE
  GITHUB_REPOSITORY GITHUB_REF GITHUB_REF_NAME GITHUB_SHA GITHUB_EVENT_NAME GITHUB_TOKEN
  GH_TOKEN GH_REPO RUNNER_OS RUNNER_TEMP
  GOFLAGS GOPATH GOCACHE GOMODCACHE XDG_CACHE_HOME DOCKER_HOST
)

# @description Record a pid this case started, for teardown.
# @arg $1 pid
function remember_pid() {
  printf '%s\n' "$1" >> "${STARTED}"
}

# @description Drop a pid from the STARTED file once its process is known to
#              have ended.
# @arg $1 pid
function forget_pid() {
  local -a kept=()
  local pid
  while IFS= read -r pid; do
    [[ "${pid}" == "$1" ]] || kept+=("${pid}")
  done < "${STARTED}"
  if ((${#kept[@]} > 0)); then
    printf '%s\n' "${kept[@]}" > "${STARTED}"
  else
    : > "${STARTED}"
  fi
}

# @description Poll until a file exists and is non-empty.
# @arg $1 path
# @exitcode 0 it appeared
# @exitcode 1 it did not appear in time
function wait_for_file() {
  local ticks=0
  while [[ ! -s "$1" ]]; do
    ((++ticks > WAIT_TICKS)) && return 1
    sleep 0.1
  done
}

# @description Poll until a pid stops answering `kill -0`, then forget it.
# @arg $1 pid
# @exitcode 0 it is gone
# @exitcode 1 it is still there after the timeout
function wait_for_exit() {
  local ticks=0
  while kill -0 "$1" 2> '/dev/null'; do
    ((++ticks > WAIT_TICKS)) && return 1
    sleep 0.1
  done
  forget_pid "$1"
}

# @description Fail unless a recorded command has ended; then forget its pid.
# @arg $1 pid
function assert_gone() {
  if kill -0 "$1" 2> '/dev/null'; then
    fail "the command (pid $1) is still running"
  fi
  forget_pid "$1"
}

# @description Fail unless a directory is empty, naming what was left in it.
# @arg $1 directory
function assert_empty_dir() {
  local left
  left="$(find "$1" -mindepth 1 -maxdepth 1 -printf '%f ')"
  if [[ -n "${left}" ]]; then
    fail "left behind in $1: ${left}"
  fi
}

# @description Run the wrapper outside the devShell boundary with the given
#              TMPDIR, against the real nix, stdin from /dev/null.
# @arg $1 tmpdir the TMPDIR the caller hands the wrapper
# @arg $@ the command to wrap
function wrap() {
  local -r tmpdir="$1"
  shift
  env --unset=IN_DEVSHELL NIX_CONFIG='extra-experimental-features = nix-command flakes' \
    TMPDIR="${tmpdir}" "${WRAPPER}" "$@" < '/dev/null'
}

# @description Run the wrapper outside the devShell boundary with SCRATCH as
#              its TMPDIR, against the stub nix.
# @arg $@ the command to wrap
function wrap_stub() {
  env --unset=IN_DEVSHELL PATH="${STUB_DIR}:${PATH}" TMPDIR="${SCRATCH}" "${WRAPPER}" "$@" < '/dev/null'
}

# @description Print the command line the wrapper builds for nix, up to and
#              including --command.
# @arg $1 flake the flake reference expected
function expected_nix_args() {
  local args="develop $1 --ignore-environment"
  local keep_var
  for keep_var in "${KEEP_VARS[@]}"; do
    args+=" --keep ${keep_var}"
  done
  printf '%s --command' "${args}"
}

@test 'exits 1 with usage when given no command' {
  run env --unset=IN_DEVSHELL "${WRAPPER}"
  assert_failure 1
  assert_output --partial 'usage: in-devshell <command>'
}

@test 'runs the command directly when IN_DEVSHELL is set' {
  IN_DEVSHELL=1 TMPDIR="${SCRATCH}" run "${WRAPPER}" echo MARKER
  assert_success
  assert_output 'MARKER'
  assert_empty_dir "${SCRATCH}"
}

@test 'enters the devShell of its own repo through nix develop' {
  make_repo
  run wrap_stub echo hi
  assert_success
  assert_line --index 0 "nix-ran $(expected_nix_args "${REPO_DIR}") echo hi"
  assert_line --index 1 'IN_DEVSHELL=1'
  assert_line --index 2 --regexp "^TMPDIR=${SCRATCH}/in-devshell\.[^/]+$"
  assert_empty_dir "${SCRATCH}"
}

@test 'with GIT_DIR set, as git sets it for a hook, still enters its own repo' {
  local git_dir
  git_dir="$(git -C "${REPO_DIR}" rev-parse --absolute-git-dir)"
  mkdir -- "${BATS_TEST_TMPDIR}/elsewhere"
  cd "${BATS_TEST_TMPDIR}/elsewhere"
  GIT_DIR="${git_dir}" run wrap_stub true
  assert_success
  assert_line --index 0 "nix-ran $(expected_nix_args "${REPO_DIR}") true"
}

@test 'exits 1 when its own directory is in no git repo' {
  # The ceiling stops git at the repo root, so from .ci/ it finds no repo.
  GIT_CEILING_DIRECTORIES="${REPO_DIR}" run wrap_stub true
  assert_failure 1
  assert_output --partial 'not a git repo'
  refute_output --partial 'nix-ran'
  assert_empty_dir "${SCRATCH}"
}

@test 'a caller TMPDIR naming no directory fails with one line and creates nothing' {
  local value
  cd "${BATS_TEST_TMPDIR}"
  printf 'not a directory\n' > "${BATS_TEST_TMPDIR}/a-file"
  for value in "${BATS_TEST_TMPDIR}/does-not-exist" 'does-not-exist-relative' "${BATS_TEST_TMPDIR}/a-file"; do
    run env --unset=IN_DEVSHELL PATH="${STUB_DIR}:${PATH}" TMPDIR="${value}" "${WRAPPER}" true
    assert_failure 1
    assert_equal "${#lines[@]}" 1
    assert_output "in-devshell: TMPDIR ${value} names no directory"
  done
  assert [ ! -e "${BATS_TEST_TMPDIR}/does-not-exist" ]
  assert [ ! -e "${BATS_TEST_TMPDIR}/does-not-exist-relative" ]
}

@test 'a wrapped true succeeds and leaves TMPDIR empty' {
  cd "${REPO_DIR}"
  run wrap "${SCRATCH}" true
  assert_success
  assert_empty_dir "${SCRATCH}"
}

@test "the command's exit status is the wrapper's" {
  cd "${REPO_DIR}"
  run wrap "${SCRATCH}" sh -c 'exit 3'
  assert_failure 3
  assert_empty_dir "${SCRATCH}"
}

@test 'the command sees a private TMPDIR inside the caller one' {
  cd "${REPO_DIR}"
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  run wrap "${SCRATCH}" sh -c 'printf "TMPDIR=%s\n" "${TMPDIR}"'
  assert_success
  assert_line --regexp "^TMPDIR=${SCRATCH}/.+"
  assert_empty_dir "${SCRATCH}"
}

@test 'a relative caller TMPDIR that exists is made absolute' {
  local rel
  cd "${REPO_DIR}"
  rel="$(realpath --relative-to="${REPO_DIR}" -- "${SCRATCH}")"
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  run wrap "${rel}" sh -c 'printf "TMPDIR=%s\n" "${TMPDIR}"'
  assert_success
  assert_line --regexp "^TMPDIR=${SCRATCH}/.+"
  assert_empty_dir "${SCRATCH}"
}

@test "the devShell is the wrapper's own repo, whatever the caller's cwd" {
  # A directory outside any git repository: the wrapper finds its flake from
  # where it lives, not from where it is called.
  mkdir -- "${BATS_TEST_TMPDIR}/elsewhere"
  cd "${BATS_TEST_TMPDIR}/elsewhere"
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  run wrap "${SCRATCH}" sh -c 'printf "IN_DEVSHELL=%s\n" "${IN_DEVSHELL:-}"; pwd; exit 4'
  assert_failure 4
  assert_line 'IN_DEVSHELL=1'
  assert_line "${BATS_TEST_TMPDIR}/elsewhere"
  assert_empty_dir "${SCRATCH}"
}

# The signal cases start the wrapper as a plain background command, not through
# `wrap`: bash forks once and env execs the wrapper, so `$!` is the wrapper's
# own pid. A function run in the background would put a subshell in between.

@test 'SIGINT to the process group ends the command and leaves TMPDIR empty' {
  local wrapper_pid cmd_pid
  cd "${REPO_DIR}"
  # Job control gives the background wrapper a process group of its own, so
  # the signal below reaches only the processes this case started.
  set -m
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  env --unset=IN_DEVSHELL NIX_CONFIG='extra-experimental-features = nix-command flakes' \
    TMPDIR="${SCRATCH}" "${WRAPPER}" \
    sh -c 'echo $$ > "$1"; exec sleep 300' sh "${BATS_TEST_TMPDIR}/cmd.pid" \
    < '/dev/null' > "${BATS_TEST_TMPDIR}/out" 2>&1 3>&- &
  wrapper_pid="$!"
  set +m
  remember_pid "${wrapper_pid}"
  wait_for_file "${BATS_TEST_TMPDIR}/cmd.pid" \
    || fail "the command never started: $(< "${BATS_TEST_TMPDIR}/out")"
  cmd_pid="$(< "${BATS_TEST_TMPDIR}/cmd.pid")"
  remember_pid "${cmd_pid}"
  kill -s INT -- "-${wrapper_pid}"
  wait_for_exit "${wrapper_pid}" || fail 'the wrapper did not end'
  assert_gone "${cmd_pid}"
  assert_empty_dir "${SCRATCH}"
}

@test 'a command that traps INT outlives a Ctrl-C and keeps its TMPDIR until it exits' {
  local wrapper_pid cmd_pid
  cd "${REPO_DIR}"
  set -m
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  env --unset=IN_DEVSHELL NIX_CONFIG='extra-experimental-features = nix-command flakes' \
    TMPDIR="${SCRATCH}" "${WRAPPER}" \
    sh -c 'trap "echo caught > \"\$2\"" INT; echo $$ > "$1"; while [ ! -e "$3" ]; do sleep 0.1; done' \
    sh "${BATS_TEST_TMPDIR}/cmd.pid" "${BATS_TEST_TMPDIR}/caught" "${BATS_TEST_TMPDIR}/release" \
    < '/dev/null' > "${BATS_TEST_TMPDIR}/out" 2>&1 3>&- &
  wrapper_pid="$!"
  set +m
  remember_pid "${wrapper_pid}"
  wait_for_file "${BATS_TEST_TMPDIR}/cmd.pid" \
    || fail "the command never started: $(< "${BATS_TEST_TMPDIR}/out")"
  cmd_pid="$(< "${BATS_TEST_TMPDIR}/cmd.pid")"
  remember_pid "${cmd_pid}"
  kill -s INT -- "-${wrapper_pid}"
  wait_for_file "${BATS_TEST_TMPDIR}/caught" || fail 'the command never saw the signal'
  kill -0 "${wrapper_pid}" 2> '/dev/null' || fail 'the wrapper died on the first INT'
  kill -0 "${cmd_pid}" 2> '/dev/null' || fail 'the command died on the first INT'
  if [[ -z "$(find "${SCRATCH}" -mindepth 1 -maxdepth 1)" ]]; then
    fail 'the private TMPDIR was removed under a running command'
  fi
  : > "${BATS_TEST_TMPDIR}/release"
  wait_for_exit "${wrapper_pid}" || fail 'the wrapper did not end after the command did'
  assert_gone "${cmd_pid}"
  assert_empty_dir "${SCRATCH}"
}

@test 'SIGTERM to the wrapper alone gives 143 and leaves TMPDIR empty' {
  local wrapper_pid cmd_pid rc=0
  cd "${REPO_DIR}"
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  env --unset=IN_DEVSHELL NIX_CONFIG='extra-experimental-features = nix-command flakes' \
    TMPDIR="${SCRATCH}" "${WRAPPER}" \
    sh -c 'echo $$ > "$1"; exec sleep 300' sh "${BATS_TEST_TMPDIR}/cmd.pid" \
    < '/dev/null' > "${BATS_TEST_TMPDIR}/out" 2>&1 3>&- &
  wrapper_pid="$!"
  remember_pid "${wrapper_pid}"
  wait_for_file "${BATS_TEST_TMPDIR}/cmd.pid" \
    || fail "the command never started: $(< "${BATS_TEST_TMPDIR}/out")"
  cmd_pid="$(< "${BATS_TEST_TMPDIR}/cmd.pid")"
  remember_pid "${cmd_pid}"
  kill -s TERM "${wrapper_pid}"
  wait "${wrapper_pid}" || rc="$?"
  forget_pid "${wrapper_pid}"
  assert_equal "${rc}" 143
  assert_gone "${cmd_pid}"
  assert_empty_dir "${SCRATCH}"
}

@test 'a command that handles a forwarded SIGTERM reports its own status' {
  local wrapper_pid cmd_pid rc=0
  cd "${REPO_DIR}"
  # shellcheck disable=SC2016 # expanded by the wrapped sh, not here
  env --unset=IN_DEVSHELL NIX_CONFIG='extra-experimental-features = nix-command flakes' \
    TMPDIR="${SCRATCH}" "${WRAPPER}" \
    sh -c 'trap "kill \$! 2> /dev/null; exit 0" TERM; echo $$ > "$1"; sleep 300 & wait' \
    sh "${BATS_TEST_TMPDIR}/cmd.pid" \
    < '/dev/null' > "${BATS_TEST_TMPDIR}/out" 2>&1 3>&- &
  wrapper_pid="$!"
  remember_pid "${wrapper_pid}"
  wait_for_file "${BATS_TEST_TMPDIR}/cmd.pid" \
    || fail "the command never started: $(< "${BATS_TEST_TMPDIR}/out")"
  cmd_pid="$(< "${BATS_TEST_TMPDIR}/cmd.pid")"
  remember_pid "${cmd_pid}"
  kill -s TERM "${wrapper_pid}"
  wait "${wrapper_pid}" || rc="$?"
  forget_pid "${wrapper_pid}"
  assert_equal "${rc}" 0
  assert_gone "${cmd_pid}"
  assert_empty_dir "${SCRATCH}"
}
