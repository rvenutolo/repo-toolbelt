#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Stubs stand in for every linter and for .ci/vuln; each prints a marker and
# the arguments it received, so no real linter runs. run-lint-checks lists
# files through git, so the fixture commits one file of each kind it selects.

# @description Write an executable stub that prints "<name>-ran <args>" and
#              exits with $<NAME>_EXIT, defaulting to 0.
# @arg $1 path where the stub is written
# @arg $2 name the stub reports
# @arg $3 exit_var name of the environment variable holding the exit status
function write_stub() {
  local -r path="$1"
  local -r name="$2"
  local -r exit_var="$3"
  # shellcheck disable=SC2016 # the stub's own expansions must stay literal in the file it writes
  printf '#!/usr/bin/env bash\nprintf "%s-ran %%s\\n" "$*"\nexit "${%s:-0}"\n' "${name}" "${exit_var}" > "${path}"
  chmod +x "${path}"
}

# @description Commit everything in the fixture.
# @noargs
function commit_all() {
  git add --all
  git -c user.name=t -c user.email=t@t commit --quiet --message fixture
}

# @description Untrack paths, keeping them on disk, and commit.
# @arg $@ paths tracked files to drop from the index
function untrack_and_commit() {
  git rm --quiet --cached -- "$@"
  git -c user.name=t -c user.email=t@t commit --quiet --message 'untrack'
}

# @description Assert the last run failed because one file list was empty
#              and that a later linter still ran.
# @arg $1 label the list's name as the failure message spells it
# @arg $2 later_tool marker of a linter that runs after that step
function assert_empty_list_failure() {
  local -r label="$1"
  local -r later_tool="$2"
  assert_failure 1
  assert_output --partial "no ${label} found; this is a failure, not a clean pass"
  assert_output --partial "${later_tool}-ran"
  assert_output --partial 'one or more linters failed'
}

function setup() {
  load test_helper/common
  make_repo
  STUB_BIN="${BATS_TEST_TMPDIR}/bin"
  mkdir "${STUB_BIN}" .ci .coverage test .github .github/workflows
  write_stub "${STUB_BIN}/shellcheck" shellcheck SHELLCHECK_EXIT
  # shfmt is stubbed so a run of it would show: the shell formatter's
  # check-shell-formatted holds shell formatting, not this script.
  write_stub "${STUB_BIN}/shfmt" shfmt SHFMT_EXIT
  write_stub "${STUB_BIN}/just" just JUST_EXIT
  write_stub "${STUB_BIN}/statix" statix STATIX_EXIT
  write_stub "${STUB_BIN}/deadnix" deadnix DEADNIX_EXIT
  write_stub "${STUB_BIN}/actionlint" actionlint ACTIONLINT_EXIT
  write_stub "${STUB_BIN}/zizmor" zizmor ZIZMOR_EXIT
  write_stub "${STUB_BIN}/yamllint" yamllint YAMLLINT_EXIT
  write_stub "${STUB_BIN}/markdownlint-cli2" markdownlint-cli2 MARKDOWNLINT_EXIT
  write_stub "${STUB_BIN}/golangci-lint" golangci-lint GOLANGCI_LINT_EXIT
  write_stub "${STUB_BIN}/go" go GO_EXIT
  write_stub "${STUB_BIN}/typos" typos TYPOS_EXIT
  write_stub .ci/vuln vuln VULN_EXIT
  write_stub .ci/x ci-x CI_X_EXIT
  write_stub .coverage/y coverage-y COVERAGE_Y_EXIT
  write_stub tool tool TOOL_EXIT
  printf '# test\n' > test/a.bats
  : > .justfile
  : > a.nix
  : > .github/workflows/w.yml
  : > x.yaml
  printf '# readme\n' > README.md
  printf 'plain text\n' > notes.txt
  commit_all
  # Only the stub dir goes ahead of the devShell PATH.
  PATH="${STUB_BIN}:${PATH}"
}

@test 'runs every linter and passes' {
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_success
  local -a expected=(
    shellcheck just statix deadnix actionlint zizmor yamllint markdownlint-cli2
    golangci-lint golangci-lint go go vuln typos
  )
  local -a ran=()
  local line
  for line in "${lines[@]}"; do
    if [[ "${line}" == *-ran\ * || "${line}" == *-ran ]]; then
      ran+=("${line%%-ran*}")
    fi
  done
  [[ "${ran[*]}" == "${expected[*]}" ]]
}

@test 'shellcheck gets the shell files and skips .coverage' {
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_success
  assert_line 'shellcheck-ran --external-sources .ci/vuln .ci/x test/a.bats tool'
  local line
  for line in "${lines[@]}"; do
    [[ "${line}" != shellcheck-ran*'.coverage/y'* ]]
  done
}

@test 'an extensionless executable under formatters/ reaches shellcheck' {
  mkdir --parents formatters/shell
  write_stub formatters/shell/shell fmt-shell FMT_SHELL_EXIT
  commit_all
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_success
  assert_line 'shellcheck-ran --external-sources .ci/vuln .ci/x formatters/shell/shell test/a.bats tool'
}

@test 'each linter gets only its kind' {
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_success
  assert_line 'just-ran --fmt --check --unstable --justfile .justfile'
  assert_line 'statix-ran check a.nix'
  assert_line 'deadnix-ran --fail a.nix'
  assert_line 'actionlint-ran .github/workflows/w.yml'
  assert_line 'yamllint-ran --strict .github/workflows/w.yml x.yaml'
  assert_line 'markdownlint-cli2-ran README.md'
}

@test 'typos gets every tracked text file' {
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_success
  local -a text_files=(
    .ci/vuln .ci/x .coverage/y .github/workflows/w.yml README.md a.nix notes.txt test/a.bats tool x.yaml
  )
  local -a tracked=()
  local file
  for file in "${text_files[@]}"; do
    [[ -s "${file}" ]] && tracked+=("${file}")
  done
  assert_line "typos-ran ${tracked[*]}"
}

@test 'an empty file list fails that step' {
  git rm --quiet a.nix
  git -c user.name=t -c user.email=t@t commit --quiet --message 'drop nix'
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_failure 1
  assert_output --partial 'no nix files found; this is a failure, not a clean pass'
  refute_output --partial 'statix-ran'
  refute_output --partial 'deadnix-ran'
  assert_output --partial 'typos-ran'
}

@test 'a failing linter fails the run and the rest still run' {
  SHELLCHECK_EXIT=1 run "${REPO_DIR}/.ci/run-lint-checks"
  assert_failure 1
  assert_output --partial 'typos-ran'
  assert_output --partial 'one or more linters failed'
}

@test 'a failing vuln scan fails the run' {
  VULN_EXIT=1 run "${REPO_DIR}/.ci/run-lint-checks"
  assert_failure 1
  assert_output --partial 'one or more linters failed'
}

@test 'no shell files at all fails that step and the rest still run' {
  untrack_and_commit .ci/vuln .ci/x tool test/a.bats
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_empty_list_failure 'shell files' typos
  refute_output --partial 'shellcheck-ran'
}

@test 'a missing justfile fails that step and the rest still run' {
  untrack_and_commit .justfile
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_empty_list_failure 'justfile' typos
  refute_output --partial 'just-ran'
}

@test 'no workflow files fails the workflow and .github steps' {
  untrack_and_commit .github/workflows/w.yml
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_empty_list_failure 'workflow files' typos
  assert_output --partial 'no files under .github/ found; this is a failure, not a clean pass'
  refute_output --partial 'actionlint-ran'
  refute_output --partial 'zizmor-ran'
}

@test 'no yaml files fails that step and the rest still run' {
  untrack_and_commit .github/workflows/w.yml x.yaml
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_empty_list_failure 'yaml files' typos
  refute_output --partial 'yamllint-ran'
}

@test 'no markdown files fails that step and the rest still run' {
  untrack_and_commit README.md
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_empty_list_failure 'markdown files' typos
  refute_output --partial 'markdownlint-cli2-ran'
}

@test 'a repo whose tracked files are all empty fails the typos step' {
  untrack_and_commit .ci/vuln .ci/x .coverage/y tool
  : > test/a.bats
  : > README.md
  : > notes.txt
  git -c user.name=t -c user.email=t@t commit --quiet --all --message 'empty'
  run "${REPO_DIR}/.ci/run-lint-checks"
  assert_failure 1
  assert_output --partial 'no text files found; this is a failure, not a clean pass'
  assert_output --partial 'vuln-ran'
  refute_output --partial 'typos-ran'
}
